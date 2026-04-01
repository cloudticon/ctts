import { deployment } from "ctts/k8s/apps/v1";

deployment({
  name: "web-app",
  image: Values.image,
  replicas: Values.replicas,
  ports: [{ containerPort: 8080 }],
});
