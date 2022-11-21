eval "$GOSHIMMER_SEEDS"

export ANSIBLE_STRATEGY=free
export ANSIBLE_PIPELINING=true

ARGS=("$@")
ansible-playbook -u root -i deploy/ansible/hosts/"${1}" \
  --forks 20 --ssh-common-args "-o ControlMaster=auto -o ControlPath=/tmp/masterinstance -o ControlPersist=5m" \
  --extra-vars \
  "ANALYSISSENTRY_01_ENTRYNODE_SEED=$ANALYSISSENTRY_01_ENTRYNODE_SEED
BOOTSTRAP_01_SEED=$BOOTSTRAP_01_SEED
VANILLA_01_SEED=$VANILLA_01_SEED
NODE_01_SEED=$NODE_01_SEED
NODE_02_SEED=$NODE_02_SEED
NODE_03_SEED=$NODE_03_SEED
NODE_04_SEED=$NODE_04_SEED
NODE_05_SEED=$NODE_05_SEED
FAUCET_01_SEED=$FAUCET_01_SEED
FAUCET_01_FAUCET_SEED=$FAUCET_01_FAUCET_SEED
mongoDBUser=$MONGODB_USER
mongoDBPassword=$MONGODB_PASSWORD
assetRegistryUser=$ASSET_REGISTRY_USER
assetRegistryPassword=$ASSET_REGISTRY_PASSWORD
networkVersion=$NETWORK_VERSION
grafanaAdminPassword=$GRAFANA_ADMIN_PASSWORD
elkElasticUser=$ELK_ELASTIC_USER
elkElasticPassword=$ELK_ELASTIC_PASSWORD
slackNotificationWebhook=$SLACK_NOTIFICATION_WEBHOOK
powDifficulty=$POW_DIFFICULTY
goshimmerDockerImage=$GOSHIMMER_DOCKER_IMAGE
goshimmerDockerTag=$GOSHIMMER_DOCKER_TAG
snapshotterBucket=$SNAPSHOTTER_BUCKET
snapshotterAccessKey=$SNAPSHOTTER_ACCESS_KEY
GENESIS_TIME=$(date +%s)
snapshotterSecretKey=$SNAPSHOTTER_SECRET_KEY" \
  ${ARGS[@]:2} deploy/ansible/"${2:-deploy.yml}"
