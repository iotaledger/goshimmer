# pre-devnet environment
pre-devnet is a play & test environment where we run almost full set of Goshimmer nodes to simulate the devnet network. 

This environment is deployed automatically on every commit into "develop" branch. It should be used to play and test the Goshimmer network after merging a PR.

## Hosts and services
Here is the list of hosts and services that they run:
- metrics-01.pre-devnet.shimmer.iota.cafe:
  - MongoDB: PORT=27117
  - Prometheus: PORT=9090 (not opened to the world)
  - Grafana: PORT=3000
  - Elasticsearch: PORT=9200 (not opened to the world)
  - Logstash: PORT=5213
  - Kibana: PORT=5601
    
- analysisentry-01.pre-devnet.shimmer.iota.cafe:
  - Entrynode: autopeering.port=15626
  - Analysisserver: analysis.server.bindAddress=0.0.0.0:21888; analysis.dashboard.bindAddress=0.0.0.0:28080 
    
- bootstrap-01.pre-devnet.shimmer.iota.cafe:
  - Bootstrap Goshimmer Node: AUTOPEERING_PORT=33501; GOSSIP_PORT=33601 

- faucet-01.pre-devnet.shimmer.iota.cafe:
  - Faucet Goshimmer Node: AUTOPEERING_PORT=33501; GOSSIP_PORT=33601

- vanilla-01.pre-devnet.shimmer.iota.cafe:
  - General Goshimmer Node: AUTOPEERING_PORT=33501; GOSSIP_PORT=33601
  
- drng-01.pre-devnet.shimmer.iota.cafe:
  - General Goshimmer Node: AUTOPEERING_PORT=33501; GOSSIP_PORT=33601
  
- drng-02.pre-devnet.shimmer.iota.cafe:
  - General Goshimmer Node: AUTOPEERING_PORT=33501; GOSSIP_PORT=33601
  
- drng-03.pre-devnet.shimmer.iota.cafe:
  - General Goshimmer Node: AUTOPEERING_PORT=33501; GOSSIP_PORT=33601
  
- drng-04.pre-devnet.shimmer.iota.cafe:
  - General Goshimmer Node: AUTOPEERING_PORT=33501; GOSSIP_PORT=33601
  
- drng-05.pre-devnet.shimmer.iota.cafe:
  - General Goshimmer Node: AUTOPEERING_PORT=33501; GOSSIP_PORT=33601
  
- drand-01.pre-devnet.shimmer.iota.cafe:
  - Drand Node: PORT=1234 
  - Drand Node: PORT=2234
  - Drand Node: PORT=3234
  - Drand Node: PORT=4234
  - Drand Node: PORT=5234
