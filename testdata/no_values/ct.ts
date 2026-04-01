import { deployment } from "ctts/k8s/apps/v1";

deployment({
  name: "static-app",
  image: "nginx:latest",
  replicas: 2,
  ports: [{ containerPort: 80 }],
});
