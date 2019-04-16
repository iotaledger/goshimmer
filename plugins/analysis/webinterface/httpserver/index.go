package httpserver

import (
    "fmt"
    "net/http"
)

func index(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, `<head>
  <style> body { margin: 0; } </style>

  <script src="https://unpkg.com/3d-force-graph"></script>
  <!--<script src="../../dist/3d-force-graph.js"></script>-->
</head>

<body>
  <div id="3d-graph"></div>

  <script>
	var socket = new WebSocket(((window.location.protocol === "https:") ? "wss://" : "ws://") + window.location.host + "/datastream");

	socket.onopen = function () {
		console.log("Status: Connected\n");

        setInterval(function() {
          socket.send("_");
        }, 1000);
	};

	socket.onmessage = function (e) {
        switch (e.data[0]) {
          case "_":
            // do nothing - its just a ping
          break;

          case "A":
            addNode(e.data.substr(1));
          break;

          case "a":
             removeNode(e.data.substr(1));
          break;

          case "C":
             connectNodes(e.data.substr(1, 40), e.data.substr(41, 40));
          break;

          case "c":
             disconnectNodes(e.data.substr(1, 40), e.data.substr(41, 40));
          break;

          case "O":
             setNodeOnline(e.data.substr(1));
          break;

          case "o":
             setNodeOffline(e.data.substr(1));
          break;
        }
	};

    var nodesById = {};

    const data = {
      nodes: [],
      links: []
    };

    const elem = document.getElementById("3d-graph");

    const Graph = ForceGraph3D()(elem)
        .enableNodeDrag(false)
        .onNodeHover(node => elem.style.cursor = node ? 'pointer' : null)
        .onNodeClick(removeNodeX)
        .linkDirectionalParticles(3)
        .linkDirectionalParticleWidth(0.8)
        .linkDirectionalParticleSpeed(0.01)
        .nodeColor(node => node.online ? 'rgba(0,255,0,1)' : 'rgba(255,255,255,1)')
        .graphData(data);

    function addNode(nodeId) {
      node = {id : nodeId, online: false};

      if (!(node.id in nodesById)) {
        data.nodes = [...data.nodes, node];

        nodesById[node.id] = node;

        Graph.graphData(data);
      }
    }

    function removeNode(nodeId) {
      data.links = data.links.filter(l => l.source.id !== nodeId && l.target.id !== nodeId);
      data.nodes = data.nodes.filter(currentNode => currentNode.id != nodeId)

      delete nodesById[nodeId];

      Graph.graphData(data);
    }

    function setNodeOnline(nodeId) {
      nodesById[nodeId].online = true;

      Graph.graphData(data);
    }

    function setNodeOffline(nodeId) {
      nodesById[nodeId].online = false;

      Graph.graphData(data);
    }

    function connectNodes(sourceNodeId, targetNodeId) {
      data.links = [...data.links, { source: sourceNodeId, target: targetNodeId }];

      Graph.graphData(data);
    }

    function disconnectNodes(sourceNodeId, targetNodeId) {
      data.links = data.links.filter(l => !(l.source.id == sourceNodeId && l.target.id == targetNodeId) && !(l.source.id == targetNodeId && l.target.id == sourceNodeId));

      Graph.graphData(data);
    }

    function removeNodeX(node) {
      removeNode(node.id)
    }
  </script>
</body>`)
}
