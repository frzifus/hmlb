# vLLM

Deploy [vllm](https://github.com/vllm-project/vllm) with the [IBM Granite](https://huggingface.co/ibm-granite) model, signed by [Sigstore](https://www.sigstore.dev/) using the Sigstore [model transparency cli](https://github.com/sigstore/model-transparency/) and the [model validation operator](https://github.com/sigstore/model-validation-operator/).

## Model Validation CR

The Model Validation Operator will use the [demo Granite Validation CR](granite-validation.yaml). We need to provide an identity and issuer there.

Google example:
- Identity "fake@gmail.com"
- Issuer "https://accounts.google.com"

Github example:
- Identity "fake@example.com"
- Issuer "https://github.com/login/oauth"

```diff
apiVersion: rhtas.redhat.com/v1alpha1
kind: ModelValidation
metadata:
  name: demo
  namespace: llm
spec:
  config:
    sigstoreConfig:
+      certificateIdentity: "fake@gmail.com"
+      certificateOidcIssuer: "https://accounts.google.com"
...
```

## Automatic Model Validation

When a pod spec provides the following label:
```yaml
      labels:
        validation.rhtas.redhat.com/ml: "true"
```
The Model Validation Operator will patch the created workload with an init-container to validate the integrity of the model configured on the `demo` custom resource. Since `vllm` is labeld accordingly in this setup, restarting the workload will trigger another evaluation.

This can be achieved with the following command:
```bash
oc rollout restart deployment vllm
```

Afterwards we will see the init-container in aciton. (Using `oc describe pod <podname>` we can inspect all the details).
```bash
$ oc get pods              
NAME                                      READY   STATUS     RESTARTS      AGE
llamastack-5586bf4845-5vgfc               1/1     Running    0             5d22h
llamastack-playground-57fd659797-wjchr    1/1     Running    1 (24h ago)   11d
model-validation-debug-85848698fd-w9f25   1/1     Running    0             111m
open-webui-657884dd87-bs2r2               1/1     Running    0             20h
vllm-5ccc55f587-bf449                     0/1     Init:0/1   0             4s   <------
```

To gain more insights with our validation work or why it failed, we can check the logs of the added `model-validation` container on our `vllm` pod.
```bash
$ oc logs vllm-5ccc55f587-bf449 -c model-validation
Key 6f260089d5923daf20166ca657c543af618346ab971884a99962b01988bbe0c3 failed to verify root
Key 22f4caec6d8e6f9555af66b3d4c3cb06a3bb23fdc7e39c916c61f462e6f52b06 failed to verify root
Verification succeeded
```

## Debug

Use the [model-validation-debug container](granite-validation-debug.yaml) to `sign`, `validate`, `delete signature`, `modify` and `restore` the IBM Granite model.
```bash
$ oc get pods                
NAME                                      READY   STATUS    RESTARTS      AGE
llamastack-5586bf4845-5vgfc               1/1     Running   0             5d22h
llamastack-playground-57fd659797-wjchr    1/1     Running   1 (23h ago)   11d
model-validation-debug-85848698fd-w9f25   1/1     Running   0             73m
open-webui-657884dd87-bs2r2               1/1     Running   0             20h
vllm-69d4955fb9-fv92t                     1/1     Running   0             28m
```

### Sign Model

Run the following command to sign the model using the public available Sigstore instance. (Pod name may changes).

```bash
oc exec -it model-validation-debug-85848698fd-w9f25 --  model_signing sign \
       sigstore /models/huggingface/hub/models--ibm-granite--granite-3.3-2b-instruct/blobs/ \
       --signature /models/huggingface/hub/models--ibm-granite--granite-3.3-2b-instruct/model.sig
```

### Verify Model

The same debug container can be used to validate the model.

```bash
oc exec -it model-validation-debug-85848698fd-w9f25 -- model_signing verify  \
    sigstore /models/huggingface/hub/models--ibm-granite--granite-3.3-2b-instruct/blobs/ \
    --signature /models/huggingface/hub/models--ibm-granite--granite-3.3-2b-instruct/model.sig \
    --identity_provider "https://accounts.google.com" \
    --identity "<your-email>"
```

### Modify and Restore Model

To simulate unwanted modifications, we can modify the model manually.
```bash
oc exec -it model-validation-debug-85848698fd-w9f25 -- sh -c 'echo "fake" > /models/huggingface/hub/models--ibm-granite--granite-3.3-2b-instruct/blobs/b0d40f9bebc505fca54f7e1af51b6e755f2807a6'
```

The `custom-backup` folder contains a backup to restore the model.
```bash
oc exec -it model-validation-debug-85848698fd-w9f25 -- sh -c 'cp /models/huggingface/hub/models--ibm-granite--granite-3.3-2b-instruct/custom-backup/b0d40f9bebc505fca54f7e1af51b6e755f2807a6 /models/huggingface/hub/models--ibm-granite--granite-3.3-2b-instruct/blobs/b0d40f9bebc505fca54f7e1af51b6e755f2807a6'
```

## Presentation

https://docs.google.com/presentation/d/191_9AGpNxtU8JNIQmHgN6DgyM3NV6doHdAhAnmLPjc8/edit#slide=id.p