import { webApp } from "github.com/test/webapp";
import { configMap } from "ctts/k8s/core/v1";

const app = webApp({
  name: "frontend",
  image: "nginx:1.25",
  domain: "app.example.com",
  replicas: 2,
});

configMap({
  name: "frontend-config",
  data: { API_URL: "https://api.example.com" },
});
