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
        setInterval(function() {
          socket.send("_");
        }, 1000);
	};

	socket.onmessage = function (e) {
        console.log("Len: ", data.nodes.length);
        switch (e.data[0]) {
          case "_":
            // do nothing - its just a ping
          break;

          case "A":
             addNode(e.data.substr(1));
             console.log("Add node:",e.data.substr(1));
          break;

          case "a":
             removeNode(e.data.substr(1));
             console.log("Remove node:", e.data.substr(1));
          break;

          case "C":
             connectNodes(e.data.substr(1, 64), e.data.substr(65, 128));
             console.log("Connect nodes:",e.data.substr(1, 64), " - ", e.data.substr(65, 128));
          break;

          case "c":
             disconnectNodes(e.data.substr(1, 64), e.data.substr(65, 128));
             console.log("Disconnect nodes:",e.data.substr(1, 64), " - ", e.data.substr(65, 128));
          break;

          case "O":
             setNodeOnline(e.data.substr(1));
             console.log("setNodeOnline:",e.data.substr(1));
          break;

          case "o":
             setNodeOffline(e.data.substr(1));
             console.log("setNodeOffline:",e.data.substr(1));
          break;
        }
	};

    var nodesById = {};

    const data = {
      nodes: [],
      links: []
    };

    var existingLinks = {};

    const elem = document.getElementById("3d-graph");

    const Graph = ForceGraph3D()(elem)
        .enableNodeDrag(false)
        .onNodeHover(node => elem.style.cursor = node ? 'pointer' : null)
        .onNodeClick(removeNodeX)
        .nodeColor(node => node.online ? 'rgba(0,255,0,1)' : 'rgba(255,255,255,1)')
        .graphData(data);

    var updateRequired = true;

    setInterval(function() {
      if (updateRequired) {
        Graph.graphData(data);

        updateRequired = false;
      }
    }, 500)

    updateGraph = function() {
      updateRequired = true;
    };

    function addNode(nodeId, displayImmediately) {
      node = {id : nodeId, online: false};

      if (!(node.id in nodesById)) {
        data.nodes = [...data.nodes, node];

        nodesById[node.id] = node;
        nodesById[nodeId].online = true;

        updateGraph();
      }
    }

    function removeNode(nodeId) {
      data.links = data.links.filter(l => l.source.id !== nodeId && l.target.id !== nodeId);
      data.nodes = data.nodes.filter(currentNode => currentNode.id != nodeId)

      delete nodesById[nodeId];

      updateGraph();
    }

    function setNodeOnline(nodeId) {
      if (nodeId in nodesById) {
        nodesById[nodeId].online = true;
      }

      updateGraph();
    }

    function setNodeOffline(nodeId) {
      if (nodeId in nodesById) {
        nodesById[nodeId].online = false;

        updateGraph();
      }
    }

    function connectNodes(sourceNodeId, targetNodeId) {
      if(existingLinks[sourceNodeId + targetNodeId] == undefined && existingLinks[targetNodeId + sourceNodeId] == undefined) {
        if (!(sourceNodeId in nodesById)) {
          addNode(sourceNodeId);
        }
        if (!(targetNodeId in nodesById)) {
          addNode(targetNodeId);
        }
        nodesById[sourceNodeId].online = true;
        nodesById[targetNodeId].online = true;
        data.links = [...data.links, { source: sourceNodeId, target: targetNodeId }];

        updateGraph();
      }
    }

    function disconnectNodes(sourceNodeId, targetNodeId) {
      data.links = data.links.filter(l => !(l.source.id == sourceNodeId && l.target.id == targetNodeId) && !(l.source.id == targetNodeId && l.target.id == sourceNodeId));
      delete existingLinks[sourceNodeId + targetNodeId];
      delete existingLinks[targetNodeId + sourceNodeId];

      updateGraph();
    }

    function removeNodeX(node) {
      if (!(node.id in nodesById)) {
        addNode(sourceNodeId);
      }
      removeNode(node.id)
    }
  </script>
</body>`)
}
