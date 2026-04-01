import { deployment } from "ctts/k8s/apps/v1";
import { service } from "ctts/k8s/core/v1";
import { ingress } from "ctts/k8s/networking/v1";
import { defaults } from "./lib/defaults";

export interface WebAppOpts {
  name: string;
  image: string;
  domain?: string;
  replicas?: number;
}

export function webApp(opts: WebAppOpts) {
  const replicas = opts.replicas ?? defaults.replicas;
  const app = deployment({
    name: opts.name,
    image: opts.image,
    replicas: replicas,
    ports: [{ containerPort: defaults.port }],
  });
  service({
    name: `${opts.name}-svc`,
    selector: { app: opts.name },
    ports: [{ port: defaults.port, targetPort: defaults.port }],
  });
  if (opts.domain) {
    ingress({
      name: `${opts.name}-ing`,
      host: opts.domain,
      serviceName: `${opts.name}-svc`,
    });
  }
  return app;
}
