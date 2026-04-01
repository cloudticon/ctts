import { resource } from "ctts/k8s/resource";

resource({
  apiVersion: "redis.redis.opstreelabs.in/v1beta2",
  kind: "Redis",
  metadata: { name: "my-redis" },
  spec: {
    kubernetesConfig: { image: "redis:7.2" },
    redisExporter: { enabled: true },
  },
});
