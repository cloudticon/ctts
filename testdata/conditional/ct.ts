import { deployment } from "ctts/k8s/apps/v1";
import { service } from "ctts/k8s/core/v1";
import { ingress } from "ctts/k8s/networking/v1";

const app = deployment({
  name: "api",
  image: Values.image,
  replicas: Values.replicas,
  ports: [{ containerPort: 3000 }],
});

service({
  name: "api-svc",
  selector: { app: app.metadata.name },
  ports: [{ port: 80, targetPort: 3000 }],
});

if (Values.enableIngress) {
  ingress({
    name: "api-ingress",
    host: Values.domain,
    serviceName: "api-svc",
  });
}
