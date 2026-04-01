export {
  resource,
  resourceClusterScope,
  type ClusterScopedResourceManifest,
  type ResourceManifest,
  type ResourceMetadata,
} from "./resource";

export {
  daemonSet,
  deployment,
  statefulSet,
  type ContainerPort,
  type DaemonSetOpts,
  type DeploymentOpts,
  type EnvVar,
  type ResourceRequirements,
  type StatefulSetOpts,
  type Volume,
  type VolumeMount,
} from "./apps/v1";

export {
  configMap,
  namespace,
  secret,
  service,
  type ConfigMapOpts,
  type NamespaceOpts,
  type SecretOpts,
  type ServiceOpts,
  type ServicePort,
} from "./core/v1";

export {
  ingress,
  type IngressOpts,
  type IngressPath,
  type IngressRule,
  type IngressTLS,
} from "./networking/v1";
