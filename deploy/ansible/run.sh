eval "$GOSHIMMER_SEEDS"
ansible-playbook -u root -i deploy/ansible/"${1}" --extra-vars \
  "ANALYSISSENTRY_01_ENTRYNODE_SEED=$ANALYSISSENTRY_01_ENTRYNODE_SEED
BOOTSTRAP_01_SEED=$BOOTSTRAP_01_SEED
VANILLA_01_SEED=$VANILLA_01_SEED
DRNG_01_SEED=$DRNG_01_SEED
DRNG_02_SEED=$DRNG_02_SEED
DRNG_03_SEED=$DRNG_03_SEED
DRNG_04_SEED=$DRNG_04_SEED
DRNG_05_SEED=$DRNG_05_SEED
DRNG_XTEAM_01_SEED=$DRNG_XTEAM_01_SEED
FAUCET_01_SEED=$FAUCET_01_SEED
FAUCET_01_FAUCET_SEED=$FAUCET_01_FAUCET_SEED
drandsSecret=$DRANDS_SECRET
mongoDBUser=$MONGODB_USER
mongoDBPassword=$MONGODB_PASSWORD
networkVersion=$NETWORK_VERSION
grafanaAdminPassword=$GRAFANA_ADMIN_PASSWORD
elkElasticUser=$ELK_ELASTIC_USER
elkElasticPassword=$ELK_ELASTIC_PASSWORD" \
  deploy/ansible/"${2:-deploy.yml}"
