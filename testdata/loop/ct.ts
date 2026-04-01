import { deployment } from "ctts/k8s/apps/v1";

for (const worker of Values.workers) {
  deployment({
    name: `worker-${worker.name}`,
    image: worker.image,
    replicas: worker.replicas,
  });
}
