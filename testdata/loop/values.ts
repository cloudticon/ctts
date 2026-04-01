export default {
  workers: [
    { name: "email", image: "worker:1.0", replicas: 2 },
    { name: "pdf", image: "worker:1.0", replicas: 1 },
  ],
};
