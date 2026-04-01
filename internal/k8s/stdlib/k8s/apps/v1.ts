import { resource, ResourceManifest } from "../resource";

export interface ContainerPort {
  containerPort: number;
  name?: string;
  protocol?: string;
}

export interface EnvVar {
  name: string;
  value?: string;
  valueFrom?: object;
}

export interface ResourceRequirements {
  limits?: Record<string, string>;
  requests?: Record<string, string>;
}

export interface VolumeMount {
  name: string;
  mountPath: string;
  readOnly?: boolean;
  subPath?: string;
}

export interface Volume {
  name: string;
  configMap?: { name: string; items?: { key: string; path: string }[] };
  secret?: { secretName: string; items?: { key: string; path: string }[] };
  emptyDir?: object;
  persistentVolumeClaim?: { claimName: string; readOnly?: boolean };
  hostPath?: { path: string; type?: string };
}

export interface DeploymentOpts {
  name: string;
  image: string;
  replicas?: number;
  ports?: ContainerPort[];
  env?: EnvVar[];
  labels?: Record<string, string>;
  annotations?: Record<string, string>;
  command?: string[];
  args?: string[];
  resources?: ResourceRequirements;
  volumeMounts?: VolumeMount[];
  volumes?: Volume[];
  serviceAccountName?: string;
  nodeSelector?: Record<string, string>;
  tolerations?: object[];
  imagePullPolicy?: string;
}

export function deployment(opts: DeploymentOpts): ResourceManifest {
  const labels = { app: opts.name, ...opts.labels };
  return resource({
    apiVersion: "apps/v1",
    kind: "Deployment",
    metadata: {
      name: opts.name,
      labels,
      annotations: opts.annotations,
    },
    spec: {
      replicas: opts.replicas ?? 1,
      selector: { matchLabels: { app: opts.name } },
      template: {
        metadata: { labels },
        spec: {
          containers: [
            {
              name: opts.name,
              image: opts.image,
              ports: opts.ports,
              env: opts.env,
              command: opts.command,
              args: opts.args,
              resources: opts.resources,
              volumeMounts: opts.volumeMounts,
              imagePullPolicy: opts.imagePullPolicy,
            },
          ],
          volumes: opts.volumes,
          serviceAccountName: opts.serviceAccountName,
          nodeSelector: opts.nodeSelector,
          tolerations: opts.tolerations,
        },
      },
    },
  });
}

export interface StatefulSetOpts {
  name: string;
  image: string;
  replicas?: number;
  serviceName: string;
  ports?: ContainerPort[];
  env?: EnvVar[];
  labels?: Record<string, string>;
  annotations?: Record<string, string>;
  command?: string[];
  args?: string[];
  resources?: ResourceRequirements;
  volumeMounts?: VolumeMount[];
  volumes?: Volume[];
  volumeClaimTemplates?: {
    name: string;
    accessModes: string[];
    storageClassName?: string;
    storage: string;
  }[];
  serviceAccountName?: string;
  nodeSelector?: Record<string, string>;
  tolerations?: object[];
  imagePullPolicy?: string;
}

export function statefulSet(opts: StatefulSetOpts): ResourceManifest {
  const labels = { app: opts.name, ...opts.labels };
  const vcts = opts.volumeClaimTemplates?.map((vct) => ({
    metadata: { name: vct.name },
    spec: {
      accessModes: vct.accessModes,
      storageClassName: vct.storageClassName,
      resources: { requests: { storage: vct.storage } },
    },
  }));

  return resource({
    apiVersion: "apps/v1",
    kind: "StatefulSet",
    metadata: {
      name: opts.name,
      labels,
      annotations: opts.annotations,
    },
    spec: {
      replicas: opts.replicas ?? 1,
      serviceName: opts.serviceName,
      selector: { matchLabels: { app: opts.name } },
      template: {
        metadata: { labels },
        spec: {
          containers: [
            {
              name: opts.name,
              image: opts.image,
              ports: opts.ports,
              env: opts.env,
              command: opts.command,
              args: opts.args,
              resources: opts.resources,
              volumeMounts: opts.volumeMounts,
              imagePullPolicy: opts.imagePullPolicy,
            },
          ],
          volumes: opts.volumes,
          serviceAccountName: opts.serviceAccountName,
          nodeSelector: opts.nodeSelector,
          tolerations: opts.tolerations,
        },
      },
      volumeClaimTemplates: vcts,
    },
  });
}

export interface DaemonSetOpts {
  name: string;
  image: string;
  ports?: ContainerPort[];
  env?: EnvVar[];
  labels?: Record<string, string>;
  annotations?: Record<string, string>;
  command?: string[];
  args?: string[];
  resources?: ResourceRequirements;
  volumeMounts?: VolumeMount[];
  volumes?: Volume[];
  serviceAccountName?: string;
  nodeSelector?: Record<string, string>;
  tolerations?: object[];
  imagePullPolicy?: string;
}

export function daemonSet(opts: DaemonSetOpts): ResourceManifest {
  const labels = { app: opts.name, ...opts.labels };
  return resource({
    apiVersion: "apps/v1",
    kind: "DaemonSet",
    metadata: {
      name: opts.name,
      labels,
      annotations: opts.annotations,
    },
    spec: {
      selector: { matchLabels: { app: opts.name } },
      template: {
        metadata: { labels },
        spec: {
          containers: [
            {
              name: opts.name,
              image: opts.image,
              ports: opts.ports,
              env: opts.env,
              command: opts.command,
              args: opts.args,
              resources: opts.resources,
              volumeMounts: opts.volumeMounts,
              imagePullPolicy: opts.imagePullPolicy,
            },
          ],
          volumes: opts.volumes,
          serviceAccountName: opts.serviceAccountName,
          nodeSelector: opts.nodeSelector,
          tolerations: opts.tolerations,
        },
      },
    },
  });
}
