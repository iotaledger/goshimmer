package httpserver

import (
	"fmt"
	"net/http"
)

func index(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, `<head>
  <style> 
  html {
    font-family: monospace, monospace;
    line-height: 1.15;
    -webkit-text-size-adjust: 100%%;
  }
  body {
    text-align: center;
    margin: 0;
    overflow: hidden;
  }

  #nfo{
    position:absolute;
    right: 0;
    padding:10px;	
  }

  #knownPeers{
    position:relative;
    margin:0;
    font-size:14px;
    font-weight: bold;
    text-align:right;
    color:#aaa;
  }

  #avgNeighbors{
    position:relative;
    margin:0;
    font-size:13px;
    text-align:right;
    color:grey;
  }

  #graphc {
    position: absolute;
    top: 0px;
    right: 0px;
    margin:0;
    right: 0px;
  }

  #nodeId{
    position:relative;
    margin:0;
    padding:5px 0;
    font-size:13px;
    font-weight: bold;
    text-align:right;
    color:#aaa; 
  }
  
  #nodestat{
    position:relative;
    margin:0;
    font-size:12px;
    text-align:right;
    color:grey; 
  }
  
  #in, #out{
    margin:0;
    padding: 3px 0;
  }

  </style>

  <script src="https://unpkg.com/3d-force-graph"></script>
  <!--<script src="../../dist/3d-force-graph.js"></script>-->
</head>

<body>
  <div id="graphc"></div>
  <div id="nfo">
    <p id="knownPeers"></p>
    <p id="avgNeighbors"></p>
    <div id="nodeId"></div>
    <div id="nodestat">
      <p id="in"></p>
      <p id="out"></p>
    </div>
  </div>
  <script>
	var socket = new WebSocket(((window.location.protocol === "https:") ? "wss://" : "ws://") + window.location.host + "/datastream");

	socket.onopen = function () {
        setInterval(function() {
          socket.send("_");
        }, 1000);
	};

	socket.onmessage = function (e) {
        document.getElementById("knownPeers").innerHTML = "Known peers: " + data.nodes.length;
        document.getElementById("avgNeighbors").innerHTML = "Average neighbors: " + parseFloat(((data.links.length * 2) / data.nodes.length).toFixed(2));
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
             connectNodes(e.data.substr(1, 64), e.data.substr(65, 64));
             console.log("Connect nodes:",e.data.substr(1, 64), " - ", e.data.substr(65, 64));
          break;

          case "c":
             disconnectNodes(e.data.substr(1, 64), e.data.substr(65, 64));
             console.log("Disconnect nodes:",e.data.substr(1, 64), " - ", e.data.substr(65, 64));
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

    let highlightNodes = [];
    let highlightInbound = [];
    let highlightOutbound = [];
    let highlightLinks = [];
    let highlightLink = null;

    const elem = document.getElementById("graphc");

      //.onNodeHover(node => elem.style.cursor = node ? 'pointer' : null)

    const Graph = ForceGraph3D()(elem)
        .graphData(data)
        .enableNodeDrag(false)
        .onNodeClick(showNodeStat)
        .nodeColor(node =>  {
          if (highlightNodes.indexOf(node) != -1) {
            return 'rgb(255,0,0,1)'
          }
          if (highlightInbound.indexOf(node) != -1) {
            return 'rgba(0,255,100,0.6)'
          }
          if (highlightOutbound.indexOf(node) != -1)  {
            return 'rgba(0,100,255,0.6)'
          }
          else {
            return 'rgba(0,255,255,0.6)'
          }
        })
        .linkWidth(link => highlightLinks.indexOf(link) === -1 ? 1 : 3)
        .linkDirectionalParticles(link => highlightLinks.indexOf(link) === -1 ? 0 : 3)
        .linkDirectionalParticleWidth(3)
        .onNodeHover(node => {
          // no state change
          if ((!node && !highlightNodes.length) || (highlightNodes.length === 1 && highlightNodes[0] === node)) return;

          highlightNodes = node ? [node] : [];

          highlightLinks = [];
          highlightInbound = [];
          highlightOutbound = [];
          clearNodeStat();
          if (node != null) {
            highlightLinks = data.links.filter(l => (l.target.id == node.id) || (l.source.id == node.id));

            highlightLinks.forEach(function(link){
              if (link.target.id == node.id) {
                highlightInbound.push(link.source)
              }
              else {
                highlightOutbound.push(link.target)
              }
            });

            showNodeStat(node);
          }
          updateHighlight();
        })
        .onLinkHover(link => {
          // no state change
          if ((!link && !highlightLinks.length) || (highlightLinks.length === 1 && highlightLinks[0] === link)) return;

          highlightLinks = [link];
          highlightNodes = link ? [link.source, link.target] : [];

          updateHighlight();
        });

    var updateRequired = true;

    setInterval(function() {
      if (updateRequired) {
        Graph.graphData(data);

        updateRequired = false;
      }
    }, 500)

    function updateHighlight() {
      // trigger update of highlighted objects in scene
      Graph
        .nodeColor(Graph.nodeColor())
        .linkWidth(Graph.linkWidth())
        .linkDirectionalParticles(Graph.linkDirectionalParticles());
    }

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
      if(existingLinks[sourceNodeId + targetNodeId] == undefined) {
        if (!(sourceNodeId in nodesById)) {
          addNode(sourceNodeId);
        }
        if (!(targetNodeId in nodesById)) {
          addNode(targetNodeId);
        }
        nodesById[sourceNodeId].online = true;
        nodesById[targetNodeId].online = true;
        existingLinks[sourceNodeId + targetNodeId] = true
        data.links = [...data.links, { source: sourceNodeId, target: targetNodeId }];

        updateGraph();
      }
    }

    function disconnectNodes(sourceNodeId, targetNodeId) {
      data.links = data.links.filter(l => !(l.source.id == sourceNodeId && l.target.id == targetNodeId));
      delete existingLinks[sourceNodeId + targetNodeId];

      updateGraph();
    }

    function clearNodeStat() {
      document.getElementById("nodeId").innerHTML = ""
      document.getElementById("in").innerHTML = ""
      document.getElementById("out").innerHTML = ""
    }

    function showNodeStat(node) {
      document.getElementById("nodeId").innerHTML = "ID: " + node.id.substr(0, 16);

      var incoming = data.links.filter(l => (l.target.id == node.id));
      document.getElementById("in").innerHTML = "INBOUND (accepted): " + incoming.length + "<br>";
      incoming.forEach(function(link){
        document.getElementById("in").innerHTML += link.source.id.substr(0, 16) + " &rarr; NODE <br>";
      });

      var outgoing = data.links.filter(l => (l.source.id == node.id));
      document.getElementById("out").innerHTML = "OUTBOUND (chosen): " + outgoing.length + "<br>";
      outgoing.forEach(function(link){
        document.getElementById("out").innerHTML += "NODE &rarr; " + link.target.id.substr(0, 16) + "<br>";
      });
    }
  </script>
</body>`)
}
