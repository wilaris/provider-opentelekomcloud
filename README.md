# Provider OpenTelekomCloud

`provider-opentelekomcloud` is a [Crossplane](https://crossplane.io/) provider for [Open Telekom Cloud (OTC)](https://open-telekom-cloud.com/).

It allows you to provision and manage Open Telekom Cloud infrastructure using Kubernetes Custom Resources.

## Why this project exists

This provider powers an upcoming managed-services platform built on Crossplane. Running production managed services on Open Telekom Cloud means the people using it depend on the provider being predictable, safe to operate and quick to debug when something goes wrong. So we built one we'd be comfortable putting under that workload.

We looked at the existing Upjet-generated [`opentelekomcloud/provider-opentelekomcloud`](https://github.com/opentelekomcloud/provider-opentelekomcloud) and chose to write our own. Upjet-generated providers run an embedded Terraform engine on every reconcile and that architecture didn't meet the operability bar we set for a production-ready platform.

## What this provider gives you

### Catch mistakes before they reach the cloud

Every Custom Resource ships with rich validation rules that the Kubernetes API server enforces *at admission time*. Long before a controller picks up the manifest and starts spending money on Open Telekom Cloud. We use [CEL (Common Expression Language)](https://kubernetes.io/docs/reference/using-api/cel/) to express things that matter in real life:

- **Cross-field rules** - e.g. "exactly one of `vpcId`, `vpcIdRef` or `vpcIdSelector` must be set."
- **Immutability** - e.g. "`vpcId` cannot change after the resource is created." The API server rejects the bad update with a clear message; the controller never has to deal with an impossible state.
- **OTC-specific shape constraints** - encoded into the CRD itself, not buried in controller code.

This will cause errors in manifests to be identified quickly, with field-level error messages in 'kubectl apply'. No half-created resources, no surprise cloud bills from typos, no debugging why a controller is "stuck."

### GitOps safety, by default

Every cloud has fields that can't be changed after a resource is created. The question is *where in the pipeline* that constraint shows up and how clearly:

- **Upjet refuses the change at reconcile time.** The commit lands in the cluster as desired state. The controller picks it up, finds the change would require replacing the resource and parks the Managed Resource at `Synced=False` with an opaque "cannot update ... requires replacing it" error. Auto-sync happily applied the bad commit; the reconciler then quietly stalls until somebody notices and investigates.
- **We refuse it at admission.** The Kubernetes API server rejects the manifest with a clear, field-level message *"vpcId is immutable after creation"*, before it ever becomes accepted state. Argo CD or Flux see the sync fail at apply time. The previous good state is still there, untouched.

Both approaches stop short of doing the destructive thing. The real difference is when the operator finds out and what they have to do about it:

- **Earlier feedback.** The problem surfaces in `kubectl apply`, in CI's `kubectl diff` or in the sync tool's status. Not in a stuck reconciler hours after merge.
- **Clearer errors.** A field-level message from the API server points exactly at the offending field. A reconcile-time replacement refusal is wrapped through Terraform layers and tends to need digging.
- **No diverged state.** The cluster never accepts a manifest that conflicts with the cloud's reality, so what's in Git is what the reconciler is acting on. There's no gap to investigate.

Our position: a provider that catches these mistakes at the API server is the only kind worth putting under auto-sync. Not because the alternative destroys infrastructure, but because the alternative leaves operators debugging stalled reconcilers in production.

### Cached, shared Open Telekom Cloud clients

For each set of credentials, the provider builds one authenticated Open Telekom Cloud client and reuses it across every controller that needs it. No rebuilding clients per reconcile, no ten controllers independently asking the identity service for the same token. Different credentials still get their own isolated clients, they just aren't duplicated inside a credential set. Reconciles stay fast and the provider stays a good citizen of your account.

### Talks to Open Telekom Cloud directly

We call the official SDK for OTC directly, instead of embedding Terraform as a library. Fewer moving parts means faster reconciles, smaller images, a smaller dependency surface to keep patched and error messages that point at Open Telekom Cloud's API rather than at Terraform internals.


### Made for managed services, open to everyone

The provider is general-purpose: install it, point it at your Open Telekom Cloud account and use it however you'd use any Crossplane provider. It also happens to be one piece of a wider managed-services offering we're building on top of Crossplane, if that sounds interesting too, we'd love to hear from you.
