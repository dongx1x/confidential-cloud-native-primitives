use anyhow::{anyhow, Error};
use futures::{StreamExt, TryStreamExt};
use k8s_openapi::{
    api::core,
    serde_json::{from_value, json},
};
use kube::{
    api::{Patch, PatchParams, WatchEvent, WatchParams},
    Api, Client, ResourceExt,
};
use log::info;

const ANNOTATION_CCNP_REQUIRE: &str = "ccnp.cc-api.com/require";

pub struct Pod {}

impl Pod {
    #[tokio::main]
    pub async fn patch_ccnp_pods(sock: String) -> Result<(), Error> {
        let client = Client::try_default().await.unwrap();
        let api = Api::<core::v1::Pod>::all(client.clone());
        let mut stream = api.watch(&WatchParams::default(), "0").await?.boxed();

        let volume_patch = json_patch::Patch(
            from_value(json!([
                {
                    "op": "add",
                    "path": "/spec/volumes/-",
                    "value": {
                        "name": "ccnp-server-sock",
                        "hostPath": {
                            "path": "/run/ccnp/uds/ccnp-server.sock",
                            "type": "File"
                        }
                    }
                },
                {
                    "op": "add",
                    "path": "/spec/containers/0/volumeMounts/-",
                    "value": {
                        "name": "ccnp-server-sock",
                        "mountPath": "/run/ccnp/uds/ccnp-server.sock"
                    }
                }
            ]))
            .unwrap(),
        );

        while let Some(event) = stream.try_next().await? {
            match event {
                WatchEvent::Added(pod) | WatchEvent::Modified(pod) => {
                    if pod.annotations().get(ANNOTATION_CCNP_REQUIRE) == Some(&"true".to_string()) {
                        info!("patch...");
                        let pods = Api::<core::v1::Pod>::namespaced(
                            client.clone(),
                            &pod.namespace().unwrap_or("default".to_string()),
                        );
                        let patch2 = Patch::Json::<()>(volume_patch.clone());
                        match pods
                            .patch(&pod.name_any(), &PatchParams::default(), &patch2)
                            .await
                        {
                            Ok(_) => info!("ok"),
                            Err(e) => info!("error: {}", e),
                        }
                    }
                }
                _ => {}
            };
        }
        Ok(())
    }
}
