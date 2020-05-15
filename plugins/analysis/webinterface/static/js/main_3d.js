const ANALYSIS_SERVER_URL = "127.0.0.1" + "/datastream";
const NODE_ID_LENGTH = 64;
const backgroundColor = '#060D29'
const nodeColor = 'rgba(168, 208, 230, 0.8)' //#A8D0E6
const linkColor = 'rgba(255, 125, 108, 0.4)'  //#ff7d6c
const highlightNodeColor = 'rgba(168, 208, 230, 1.0)' //#A8D0E6
const highlightInColor =  'rgba(134, 194, 50, 1.0)' // #86c232
const highlightOutColor = 'rgba(191, 10, 173, 1.0)' // #bf0aad

class Frontend {
    constructor(app) {
        this.app = app;
        this.activeNode = '';
        this.searchTerm = '';
        document.addEventListener('click', (e) => {
            if (e.target.classList.contains("n")) {
                let htmlNode = e.target;
                let nodeId = htmlNode.innerHTML;

                if(htmlNode.classList.contains('active')) {
                    this.app.resetActiveNode();
                    return;
                }
                this.app.resetActiveNode();
                this.app.setActiveNode(nodeId, true);
            }
        }, false);

        this.initSearch();
        this.showSearchField();
    }

    initSearch() {
        document.getElementById("search").addEventListener('keyup', (e) => {
            let value = e.target.value.trim().toLowerCase();

            if(value === "") {
                this.resetSearch();
                return;
            }

            this.searchTerm = value;

            let results = [];
            for(let n of this.app.ds.gData.nodes) {
                if(n.id.startsWith(value)) {
                    results.push(n);
                }
            }

            this.displayNodesOnline(results);
        });

        document.getElementById("clear").addEventListener("click", (e) => {
            this.resetSearch();
        });
    }

    resetSearch() {
        document.getElementById("search").value = "";
        this.searchTerm = '';
        this.app.resetActiveNode();
    }

    showSearchField() {
        document.getElementById('searchWrapper').style.cssText = "display:block;"
    }

    // Show nodes
    displayNodesOnline(nodes) {
        let html = [];
        // When the active node gets removed, we should reset the nodeLink too
        var isActiveNodePresent = false;
        for(let n of nodes) {
            if(n.id == this.activeNode) {
                html.push('<span class="n active">' + n.id + "</span>");
                isActiveNodePresent = true;
            } else {
                html.push('<span class="n">' + n.id + "</span>");
            }
        }
        document.getElementById("nodesOnline").innerHTML = html.join("");
        if (!isActiveNodePresent) {
            // active node was removed
            this.resetActiveNode();
        }
    }

    setStatusMessage(msg) {
        document.getElementById("status").innerHTML = msg;
    }

    setStreamStatusMessage(msg) {
        document.getElementById("streamstatus").innerHTML = msg;
    }

    showNodeLinks(node, inNeighbors, outNeighbors) {
        document.getElementById("nodeId").innerHTML = node + " in:" + inNeighbors.length + " out:" + outNeighbors.length;

        let html = "incoming edges: ";
        // incoming
        for(let n of inNeighbors) {
            html += n + " &rarr; " + "NODE<br>";
        }
        if(inNeighbors.length == 0) { html += "no incoming edges!" }
        document.getElementById("in").innerHTML = html;

        html = "outgoing edges: ";
        // outgoing
        for(let n of outNeighbors) {
            html += "NODE" + " &rarr; " + n + "<br>";
        }
        if(outNeighbors.length == 0) { html += "no outgoing edges!" }
        document.getElementById("out").innerHTML = html;
    }

    setActiveNode(nodeId, updateHash=false) {
        this.activeNode = nodeId;
        // Determine neighbors of the node
        var inNeighbors = [];
        var outNeighbors = [];
        this.app.ds.gData.links.forEach(link => {
            if (link.source.id == nodeId){
                outNeighbors.push(link.target.id);
            } else if (link.target.id == nodeId){
                inNeighbors.push(link.source.id)
            }
        }
        )
        this.showNodeLinks(nodeId, inNeighbors, outNeighbors);
        this.highlightNode(nodeId);
        this.highlightLinks(nodeId);
        this.refreshGraphHighlights();
    }

    resetActiveNode() {
        this.activeNode = '';
        this.resetNodeLinks();
        this.clearHighlight();
    }

    resetNodeLinks() {
        document.getElementById("nodeId").innerHTML = "";
        document.getElementById("in").innerHTML = "";
        document.getElementById("out").innerHTML = "";
    }

    highlightNode(node) {
        this.app.graph.highlightNodes.add(node);
    }

    highlightLinks(node) {
        this.app.ds.gData.links.forEach(link => {
            if (link.source.id == node){
                this.app.graph.highlightOutLinks.add(link);
                this.app.graph.highlightOutNeighbors.add(link.target.id)
            } else if (link.target.id == node){
                this.app.graph.highlightInLinks.add(link);
                this.app.graph.highlightInNeighbors.add(link.source.id)
            }
        })
    }

    clearHighlight(){
        this.app.graph.highlightNodes.clear();
        this.app.graph.highlightInNeighbors.clear();
        this.app.graph.highlightOutNeighbors.clear();
        this.app.graph.highlightInLinks.clear();
        this.app.graph.highlightOutLinks.clear();
        this.refreshGraphHighlights();
    }

    // Refereshes rendering of graph
    refreshGraphHighlights(){
        this.app.graph.graph.refresh();
    }
}

class Datastructure {
    constructor(app) {
        this.app = app;
        this.gData = {
            nodes: [],
            links: []
        };
        this.needsDataSet = false;
    }

    addNode(idA) {
        this.gData.nodes.push({ id: idA})
        this.app.graph.setGraphData(this.gData);
        this.needsDataSet = true;
        this.app.setStreamStatusMessage("Added Node: " + idA);
    }

    removeNode(idA) {
        this.gData.links = this.gData.links.filter(l => l.source.id !== idA && l.target.id !== idA); // Remove links attached to node
        this.gData.nodes = this.gData.nodes.filter(n => n.id !== idA); // Remove node
        this.app.graph.setGraphData(this.gData);
        this.needsDataSet = true;
        this.app.setStreamStatusMessage("Removed Node: " + idA);
    }

    connectNodes(idA, idB) {
        this.gData.links.push({ source: idA, target: idB })
        this.app.graph.setGraphData(this.gData);
        this.needsDataSet = true;
        this.app.setStreamStatusMessage("Connected Nodes: " + idA + " > " + idB);

    }
    disconnectNodes(idA, idB) {
        this.gData.links = this.gData.links.filter(l => (l.source.id !== idA && l.target.id !== idB));
        this.gData.links = this.gData.links.filter(l => (l.source.id !== idB && l.target.id !== idA));
        this.app.graph.setGraphData(this.gData);
        this.needsDataSet = true;
        this.app.setStreamStatusMessage("Disconnected Nodes: " + idA + " > " + idB);
    }

    getStatusText() {
        var statusLine = "nodes online: " + this.gData.nodes.length + " - connections: " + this.gData.links.length;
        statusLine += " - average connections/node: ";
        if (this.gData.links.length === 0 || this.gData.nodes.length === 0) {
            statusLine += " - ";
        } else {
            // A connection has two ends, so counts double for nodes.
            statusLine += 2*(this.gData.links.length/this.gData.nodes.length).toFixed(2);
        }
        return statusLine;
    }
}

class Graph{
    constructor(app) {
        this.app = app
        this.highlightNodes = new Set()
        this.highlightInLinks = new Set()
        this.highlightOutLinks = new Set()
        this.highlightInNeighbors = new Set()
        this.highlightOutNeighbors = new Set()
        this.graph = ForceGraph3D()
            (document.getElementById('graphc'))
            .backgroundColor(backgroundColor)
            // Disable dragginf nodes for perfromance.
            .enableNodeDrag(false)
            //.graphData(app.ds.gData)
            .linkOpacity(1.0)
            .nodeOpacity(1.0)
            .nodeVal( node => {
                if (this.highlightNodes.has(node.id) === true)
                    return 5
                return 1
            })
            .nodeColor((node) => {
                if (this.highlightNodes.has(node.id) === true)
                    return highlightNodeColor
                if (this.highlightInNeighbors.has(node.id) === true)
                    return highlightInColor
                if (this.highlightOutNeighbors.has(node.id) === true)
                    return highlightOutColor
                return nodeColor
            })
            .linkColor((link) => {
                if (this.highlightInLinks.has(link) === true)
                    return highlightInColor
                if (this.highlightOutLinks.has(link) === true)
                    return highlightOutColor
                return linkColor
            })
            .nodeResolution(2)
            .linkWidth(1)
            //.linkDirectionalParticles(5)
            .numDimensions(2)	
            //.linkDirectionalParticleSpeed(0.01);
    }

    setGraphData(gData) {
        this.graph.graphData(gData)
    }

    getGraphData() {
        return this.graph.graphData()
    }
}

class Application {
    constructor(url) {
        this.url = url;
        this.ds = new Datastructure(this)
        this.graph = new Graph(this)
        this.frontend = new Frontend(this)
    }

    run() {
        this.initWebsocket();
        this.showStatus();
        this.showOnlineNodes();
    }

    initWebsocket() {
        this.socket = new WebSocket(
            ((window.location.protocol === "https:") ? "wss://" : "ws://") + this.url
        );
    
        this.socket.onopen = () => {
            this.pingId = setInterval(() => {
                this.socket.send("_");
            }, 1000);
        };
    
        this.socket.onerror = (e) => {
            console.error("WebSocket error observed", e);
        };

        this.socket.onclose = () => {
            clearInterval(this.pingId)
            console.log("Websocket connection closed.")
        }
    
        this.socket.onmessage = (e) => {
            let type = e.data[0];
            let data = e.data.substr(1);
            let idA = data.substr(0, NODE_ID_LENGTH);
            let idB;


            switch (type) {
                case "_":
                    //do nothing - its just a ping
                    break;
    
                case "A":
                    console.log("addNode event:", idA);
                    // filter out empty ids
                    if(idA.length == NODE_ID_LENGTH) {
                        this.ds.addNode(idA);
                    }
                    break;

                case "a":
                    console.log("removeNode event:", idA);
                    this.ds.removeNode(idA);
                    break;
    
                case "C":
                    idB = data.substr(NODE_ID_LENGTH, NODE_ID_LENGTH);
                    console.log("connectNodes event:", idA, " - ", idB);
                    this.ds.connectNodes(idA, idB);
                    break;
    
                case "c":
                    idB = data.substr(NODE_ID_LENGTH, NODE_ID_LENGTH);
                    console.log("disconnectNodes event:", idA, " - ", idB);
                    this.ds.disconnectNodes( idA, idB);
                    break;
            }
        }
    }

    showOnlineNodes() {
        setInterval(() => { 
            if(this.frontend.searchTerm.length > 0) {
                return;
            }
            this.frontend.displayNodesOnline(this.ds.gData.nodes)
        }, 300);
    }

    showStatus() {
        setInterval(() => {
            this.frontend.setStatusMessage(this.ds.getStatusText())
        }, 300);
    }

    setStreamStatusMessage(msg) {
        this.frontend.setStreamStatusMessage(msg);
    }

    setActiveNode(nodeId, updateHash=false) {
        // Display node and neighbors in div info
        this.frontend.setActiveNode(nodeId, updateHash);
    }

    resetActiveNode() {
        this.frontend.resetActiveNode();
    }
}

let app;
window.onload = () => {
    app = new Application(ANALYSIS_SERVER_URL);
    app.run()
}