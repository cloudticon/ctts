import { resource, ResourceManifest } from "../resource";

export interface IngressTLS {
  hosts?: string[];
  secretName?: string;
}

export interface IngressPath {
  path?: string;
  pathType?: string;
  serviceName: string;
  servicePort: number | string;
}

export interface IngressRule {
  host: string;
  paths: IngressPath[];
}

export interface IngressOpts {
  name: string;
  host?: string;
  serviceName?: string;
  servicePort?: number | string;
  ingressClassName?: string;
  tls?: IngressTLS[];
  rules?: IngressRule[];
  labels?: Record<string, string>;
  annotations?: Record<string, string>;
}

export function ingress(opts: IngressOpts): ResourceManifest {
  const rules = opts.rules
    ? opts.rules.map((rule) => ({
        host: rule.host,
        http: {
          paths: rule.paths.map((p) => ({
            path: p.path ?? "/",
            pathType: p.pathType ?? "Prefix",
            backend: {
              service: {
                name: p.serviceName,
                port:
                  typeof p.servicePort === "number"
                    ? { number: p.servicePort }
                    : { name: p.servicePort },
              },
            },
          })),
        },
      }))
    : opts.host && opts.serviceName
      ? [
          {
            host: opts.host,
            http: {
              paths: [
                {
                  path: "/",
                  pathType: "Prefix",
                  backend: {
                    service: {
                      name: opts.serviceName,
                      port:
                        typeof opts.servicePort === "number"
                          ? { number: opts.servicePort }
                          : opts.servicePort
                            ? { name: opts.servicePort }
                            : { number: 80 },
                    },
                  },
                },
              ],
            },
          },
        ]
      : [];

  return resource({
    apiVersion: "networking.k8s.io/v1",
    kind: "Ingress",
    metadata: {
      name: opts.name,
      labels: opts.labels,
      annotations: opts.annotations,
    },
    spec: {
      ingressClassName: opts.ingressClassName,
      tls: opts.tls,
      rules,
    },
  });
}
