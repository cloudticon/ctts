declare var globalThis: {
  __ct_resources: any[];
  Values: any;
};

export interface ResourceMetadata {
  name: string;
  namespace?: string;
  labels?: Record<string, string>;
  annotations?: Record<string, string>;
}

export interface ResourceManifest {
  apiVersion: string;
  kind: string;
  metadata: ResourceMetadata;
  [key: string]: any;
}

export interface ClusterScopedResourceManifest {
  apiVersion: string;
  kind: string;
  metadata: Omit<ResourceMetadata, "namespace">;
  [key: string]: any;
}

export function resource(manifest: ResourceManifest): ResourceManifest {
  const obj = { __ctts_scope: "namespaced", ...manifest };
  globalThis.__ct_resources.push(obj);
  return obj;
}

export function resourceClusterScope(
  manifest: ClusterScopedResourceManifest,
): ClusterScopedResourceManifest {
  const obj = { __ctts_scope: "cluster", ...manifest };
  globalThis.__ct_resources.push(obj);
  return obj;
}
