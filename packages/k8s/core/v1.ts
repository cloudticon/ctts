import {
  ClusterScopedResourceManifest,
  resource,
  resourceClusterScope,
  ResourceManifest,
} from "../resource";

export interface ServicePort {
  port: number;
  targetPort?: number | string;
  protocol?: string;
  name?: string;
  nodePort?: number;
}

export interface ServiceOpts {
  name: string;
  selector: Record<string, string>;
  ports: ServicePort[];
  type?: string;
  labels?: Record<string, string>;
  annotations?: Record<string, string>;
  clusterIP?: string;
  externalTrafficPolicy?: string;
  sessionAffinity?: string;
}

export function service(opts: ServiceOpts): ResourceManifest {
  return resource({
    apiVersion: "v1",
    kind: "Service",
    metadata: {
      name: opts.name,
      labels: opts.labels,
      annotations: opts.annotations,
    },
    spec: {
      type: opts.type,
      selector: opts.selector,
      ports: opts.ports,
      clusterIP: opts.clusterIP,
      externalTrafficPolicy: opts.externalTrafficPolicy,
      sessionAffinity: opts.sessionAffinity,
    },
  });
}

export interface ConfigMapOpts {
  name: string;
  data?: Record<string, string>;
  binaryData?: Record<string, string>;
  labels?: Record<string, string>;
  annotations?: Record<string, string>;
}

export function configMap(opts: ConfigMapOpts): ResourceManifest {
  return resource({
    apiVersion: "v1",
    kind: "ConfigMap",
    metadata: {
      name: opts.name,
      labels: opts.labels,
      annotations: opts.annotations,
    },
    data: opts.data,
    binaryData: opts.binaryData,
  });
}

export interface SecretOpts {
  name: string;
  type?: string;
  data?: Record<string, string>;
  stringData?: Record<string, string>;
  labels?: Record<string, string>;
  annotations?: Record<string, string>;
}

export function secret(opts: SecretOpts): ResourceManifest {
  return resource({
    apiVersion: "v1",
    kind: "Secret",
    metadata: {
      name: opts.name,
      labels: opts.labels,
      annotations: opts.annotations,
    },
    type: opts.type,
    data: opts.data,
    stringData: opts.stringData,
  });
}

export interface NamespaceOpts {
  name: string;
  labels?: Record<string, string>;
  annotations?: Record<string, string>;
}

export function namespace(opts: NamespaceOpts): ClusterScopedResourceManifest {
  return resourceClusterScope({
    apiVersion: "v1",
    kind: "Namespace",
    metadata: {
      name: opts.name,
      labels: opts.labels,
      annotations: opts.annotations,
    },
  });
}
