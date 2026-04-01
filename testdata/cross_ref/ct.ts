import { deployment } from "ctts/k8s/apps/v1";
import { service } from "ctts/k8s/core/v1";
import { ingress } from "ctts/k8s/networking/v1";

const app = deployment({
  name: "web-app",
  image: Values.image,
  replicas: Values.replicas,
  ports: [{ containerPort: 8080 }],
});

const svc = service({
  name: "web-app-svc",
  selector: { app: app.metadata.name },
  ports: [{ port: 80, targetPort: 8080 }],
});

ingress({
  name: "web-app-ingress",
  host: Values.domain,
  serviceName: svc.metadata.name,
});
