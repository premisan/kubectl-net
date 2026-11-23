package cmd

const defaultRedisImage = "redis:latest"

var redisCmd = newDBClientCommand(dbClientSpec{
	name:            "redis",
	binary:          "redis-cli",
	defaultImage:    defaultRedisImage,
	containerPrefix: "kubectl-net-redis",
	use:             "redis [GLOBAL_FLAGS...] [POD_NAME] [REDIS_CLI_ARGS...]",
	aliases:         []string{"redis-cli"},
	short:           "Connect to a Redis server from inside a target pod using redis-cli",
	long: `Connect to a Redis server from inside a Kubernetes pod's network namespace
using an Ephemeral Debug Container (default image: redis:latest, override via --image).

All kubectl-net flags (-n, -c, --image, --context, --kubeconfig, -v) must be placed
BEFORE POD_NAME. All arguments after POD_NAME are passed verbatim to redis-cli.

Interactive sessions automatically allocate a TTY; piped or redirected I/O automatically
switches to non-TTY mode.`,
	example: `  # Interactive Redis CLI session:
  kubectl net redis my-pod -h redis-svc.default.svc -p 6379

  # Quick connectivity check (PING) from a pod in a specific namespace:
  kubectl net redis -n production my-pod -h redis-svc -p 6379 PING

  # Connect to a TLS-enabled Redis Cluster and select DB index (-n belongs to redis-cli after POD_NAME):
  kubectl net redis -n production my-pod -h elasticache.internal -p 6379 --tls -c -n 0

  # Override debug container image (e.g. lightweight alpine or private registry):
  kubectl net redis --image redis:alpine my-pod -h redis-svc PING`,
})

func init() {
	RootCmd.AddCommand(redisCmd)
}
