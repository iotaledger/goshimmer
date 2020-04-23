const ANALYSIS_SERVER_URL = window.location.host + "/datastream";
const NODE_ID_LENGTH = 64;

// for some strange reason color formats for edges and nodes need to be different... careful!
const EDGE_COLOR_DEFAULT = "#ff7d6c40";
const EDGE_COLOR_OUTGOING = "#336db5ff";
const EDGE_COLOR_INCOMING = "#1c8d7fff";
const VERTEX_COLOR_DEFAULT = "0xa8d0e6";
const VERTEX_COLOR_ACTIVE = "0x336db5";
const VERTEX_COLOR_CONNECTED = "0x1c8d7f";
const VERTEX_SIZE = 14;

class Frontend {
    constructor(app) {
        this.app = app;
        this.activeNode = '';
        this.searchTerm = '';
        document.addEventListener('click', (e) => {
            if (hasClass(e.target, 'n')) {
                let htmlNode = e.target;
                let nodeId = htmlNode.innerHTML;

                if(hasClass(htmlNode, 'active')) {
                    this.app.resetActiveNode();
                    return;
                }

                this.app.resetActiveNode();
                this.app.setActiveNode(nodeId, true);
            }
        }, false);

        this.initSearch();
    }

    initSearch() {
        document.getElementById("search").addEventListener('keyup', (e) => {
            let value = e.target.value.trim().toLowerCase();

            if(value === "") {
                this.resetSearch();
                return;
            }

            this.searchTerm = value;
            
            let results = new Set();
            for(let n of this.app.ds.nodes) {
                if(n.startsWith(value)) {
                    results.add(n);
                }
            }
            
            if(results.size == 1) {
                // little hack to access element
                let n = results.values().next().value;
                this.app.setActiveNode(n, true);
            }

            this.displayNodes(results);
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

    setStatusMessage(msg) {
        document.getElementById("status").innerHTML = msg;
    }

    setStreamStatusMessage(msg) {
        document.getElementById("streamstatus").innerHTML = msg;
    }

    showNodeLinks(node, neighbors) {
        document.getElementById("nodeId").innerHTML = node + " in:" + neighbors.in.size + " out:" + neighbors.out.size;

        let html = "incoming edges: ";
        // incoming
        for(let n of neighbors.in) { 
            html += n + " &rarr; " + "NODE<br>";
        }
        if(neighbors.in.size == 0) { html += "no incoming edges!" }
        document.getElementById("in").innerHTML = html;

        html = "outgoing edges: ";
        // outgoing
        for(let n of neighbors.out) { 
            html += "NODE" + " &rarr; " + n + "<br>";
        }
        if(neighbors.out.size == 0) { html += "no outgoing edges!" }
        document.getElementById("out").innerHTML = html;
    }

    setActiveNode(nodeId, updateHash=false) {
        this.activeNode = nodeId;
        if(updateHash) {
            history.replaceState(null, null, '#'+nodeId);
        }

        let neighbors = this.app.ds.neighbors.get(nodeId);
        // when node doesn't have neighbors yet
        if (neighbors === undefined) {
            neighbors=this.app.ds.createNeighborsObject();
        }
        this.showNodeLinks(nodeId, neighbors);
    }

    resetActiveNode() {
        // currently active node will lose class at next refresh
        this.activeNode = '';
        history.replaceState(null, null, ' '); // reset location hash
        this.resetNodeLinks();
    }

    resetNodeLinks() {
        document.getElementById("nodeId").innerHTML = "";
        document.getElementById("in").innerHTML = "";
        document.getElementById("out").innerHTML = "";
    }

    displayNodes(nodes) {
        // this line might be a performance killer!
        nodes = Array.from(nodes).sort();
        // When the active node gets removed, we should reset the nodeLink too
        var isActiveNodePresent = false;

        let html = [];
        for(let n of nodes) {
            if(n == this.activeNode) {
                html.push('<span class="n active">' + n + "</span>");
                isActiveNodePresent = true;
            } else {
                html.push('<span class="n">' + n + "</span>");
            }
        }
        document.getElementById("nodesOnline").innerHTML = html.join("");
        if (!isActiveNodePresent) {
            // active node was removed
            this.resetActiveNode();
        }
    }
}

class Datastructure {
    constructor(app) {
        this.app = app;
        this.nodes = new Set();
        this.connections = new Set();
        this.neighbors = new Map();
    }

    getStatusText() {
        // avg = this.connections.size*2 / (this.nodesOnline-1) : -1 == entry node (always disconnected)
        return "nodes online: " + this.nodes.size + " - edges: " + this.connections.size + " - avg: " + (this.connections.size*2 / (this.nodes.size-1)).toFixed(2);
    }

    addNode(idA) {
        if(!this.nodes.has(idA)) {
            this.app.setStreamStatusMessage("Added Node: " + idA);
        }
        this.nodes.add(idA);
        this.app.graph.addVertex(idA);
        this.app.updateStatus();
        // 30 is the minimum springLength
        if (this.nodes.size > 30) {
            this.app.graph.layout.simulator.springLength(this.nodes.size)
        }
    }

    removeNode(idA) {
        this.nodes.delete(idA);
        this.app.graph.deleteVertex(idA)

        this.app.setStreamStatusMessage("Removed Node: " + idA); 
        this.app.updateStatus();
        // 30 is the minimum springLength
        if (this.nodes.size > 30) {
            this.app.graph.layout.simulator.springLength(this.nodes.size)
        }
    }

    createNeighborsObject() {
        return {
            in: new Set(),
            out: new Set(),
            isEmpty : function() {
                return this.in.size + this.out.size === 0;
            }
        }
    }

    connectNodes(con, idA, idB) {
        if(this.connections.has(con)) {
            this.app.setStreamStatusMessage("connectNodes skipped: " + idA + " > " + idB);
        } else {
            this.app.graph.addEdge(con, idA, idB);
            this.connections.add(con);

            // update datastructure for fast neighbor lookup
            if (!this.neighbors.has(idA)) {
                // Need to initialize the object
                this.neighbors.set(idA, this.createNeighborsObject());
            }

            this.neighbors.get(idA).out.add(idB);
            if (!this.neighbors.has(idB)) {
                // Need to initialize the object
                this.neighbors.set(idB, this.createNeighborsObject());
            }
            this.neighbors.get(idB).in.add(idA);

            this.app.setStreamStatusMessage("connectNodes: " + idA + " > " + idB);
            this.app.updateStatus();
        }
    }

    disconnectNodes(con, idA, idB) {
        if(this.connections.has(con)) {
            this.connections.delete(con);
            this.app.graph.deleteEdge(idA, idB);

            // update datastructure for fast neighbor lookup
            this.neighbors.get(idA).out.delete(idB);
            this.neighbors.get(idB).in.delete(idA);

            if (this.neighbors.get(idA).isEmpty()){
                this.neighbors.delete(idA);
            }

            if (this.neighbors.get(idB).isEmpty()){
                this.neighbors.delete(idB);
            }

            this.app.setStreamStatusMessage("disconnectNodes: " + idA + " > " + idB);
            this.app.updateStatus();
        } else {
            console.log("disconnectNodes skipped: ", idA, idB);
        }
    }
}

class Graph {
    constructor(app) {
        this.app = app;
        this.highlightedNodes = new Set();
        this.highlightedLinks = new Set();

        this.graph = Viva.Graph.graph();
        this.graphics = Viva.Graph.View.webglGraphics();
        this.calculator = Viva.Graph.centrality();

        this.layout = Viva.Graph.Layout.forceDirected(this.graph, {
            springLength: 30,
            springCoeff: 0.0001,
            dragCoeff: 0.02,
            gravity: -1.2
        });

        this.graphics.link((link) => {
            return Viva.Graph.View.webglLine(EDGE_COLOR_DEFAULT);
        });

        this.graphics.setNodeProgram(buildCircleNodeShader());

        this.graphics.node((node) => {
            return new WebGLCircle(VERTEX_SIZE, VERTEX_COLOR_DEFAULT);
        });

        this.renderer = Viva.Graph.View.renderer(this.graph, {
            layout: this.layout,
            graphics: this.graphics,
            container: document.getElementById('graphc'),
            renderLinks: true
        });

        this.initEvents();
    }

    updateNodeUiColor(node, color, save=true) {
        let nodeUI = this.graphics.getNodeUI(node);
        if (nodeUI != undefined) {
            nodeUI.color = color;
        }

        if(save) {
            this.highlightedNodes.add(node);
        }
    }

    updateLinkUiColor(idA, idB, color, save=true) {
        let con = this.graph.getLink(idA, idB);

        if(con != null) {
            let linkUI = this.graphics.getLinkUI(con.id);
            if (linkUI != undefined) {
                linkUI.color = parseColor(color);
            }

            if(save) {
                this.highlightedLinks.add(con.id);
            }
        }
    }

    updateLinkUiColorByLinkId(link, color, save=true) {
        let linkUI = this.graphics.getLinkUI(link);
        if (linkUI != undefined) {
            linkUI.color = parseColor(color);
        }

        if(save) {
            this.highlightedLinks.add(link);
        }
    }

    showNodeLinks(selectedNode) {
        if(this.highlightedLinks > 0 || this.highlightedNodes.size > 0) {
            // clean up display
            this.app.resetActiveNode();
        }

        this.graph.beginUpdate();

        let neighbors = this.app.ds.neighbors.get(selectedNode);

        // highlight current node
        this.updateNodeUiColor(selectedNode, VERTEX_COLOR_ACTIVE);

        // Node doesn't have any neighbors yet. nothing to be highlighted
        if (neighbors!==undefined){
            // highlight incoming connections
            for(let n of neighbors.in) {
                this.updateNodeUiColor(n, VERTEX_COLOR_CONNECTED);
                this.updateLinkUiColor(n, selectedNode, EDGE_COLOR_INCOMING);
            }

            // highlight outcoming connections
            for(let n of neighbors.out) {
                this.updateNodeUiColor(n, VERTEX_COLOR_CONNECTED);
                this.updateLinkUiColor(selectedNode, n, EDGE_COLOR_OUTGOING);
            }
        }

        this.graph.endUpdate();
        this.renderer.rerender();
    }

    initEvents() {
        this.events = Viva.Graph.webglInputEvents(this.graphics, this.graph);

        this.events.mouseEnter((node) => {
            this.app.setActiveNode(node.id)
        });

        this.events.mouseLeave(() => {
            if(this.highlightedLinks > 0 || this.highlightedNodes.size > 0) {
                // clean up display
                this.app.resetActiveNode();
            }
        });
    }

    resetPreviousColors() {
        this.graph.beginUpdate();

        for(let n of this.highlightedNodes) {
            this.updateNodeUiColor(n, VERTEX_COLOR_DEFAULT, false);
        }

        for(let l of this.highlightedLinks) {
            this.updateLinkUiColorByLinkId(l, EDGE_COLOR_DEFAULT, false);
        }

        this.graph.endUpdate();
        this.renderer.rerender();
    }

    addEdge(con, idA, idB) {
        this.graph.addLink(idA, idB, con);
    }

    deleteEdge(idA, idB) {
        let link = this.graph.getLink(idA, idB);
        this.graph.removeLink(link);
    }

    addVertex(idA) {
        this.graph.addNode(idA);
    }

    deleteVertex(idA) {
        this.graph.removeNode(idA);
    }

    render() {
        this.renderer.run();
    }
}

class Application {
    constructor(url) {
        this.url = url;
        this.frontend = new Frontend(this);
        this.ds = new Datastructure(this);
        this.graph = new Graph(this);

        this.rendered = false; // is the application already rendered?
        this.floodNew = 0;
        this.floodOld = 0;

    }

    setActiveNode(nodeId, updateHash=false) {
        this.graph.showNodeLinks(nodeId);
        this.frontend.setActiveNode(nodeId, updateHash);
    }

    resetActiveNode() {
        this.graph.resetPreviousColors()
        this.frontend.resetActiveNode();
    }

    setStatusMessage(msg) {
        if(this.rendered) {
            this.frontend.setStatusMessage(msg);
            console.log('%cStatusMessage: ' + msg, 'color: gray');
        }
    }

    setStreamStatusMessage(msg) {
        if(this.rendered) {
            this.frontend.setStreamStatusMessage(msg);
            console.log('%cStreamStatusMessage: ' + msg, 'color: gray');
        }
    }

    updateStatus() {
        this.setStatusMessage(this.ds.getStatusText());
    }

    showOnlineNodes() {
        setInterval(() => { 
            if(this.frontend.searchTerm.length > 0) {
                return;
            }
            this.frontend.displayNodes(this.ds.nodes)
        }, 300);
    }

    run() {
        let initialFloodTimerFunc = () => {
            if (this.floodNew > this.floodOld + 100) {
                this.setStreamStatusMessage("... received " + this.floodNew + " msg");
                this.floodOld = this.floodNew;
            } else {
                clearInterval(this.initialFloodTimer);
                this.startRendering();
            }
        }

        this.initialFloodTimer = setInterval(initialFloodTimerFunc, 500);
        this.initWebsocket();
    }

    startRendering() {
        // kickoff rendering
        this.rendered = true;

        this.graph.render();
        
        this.setStreamStatusMessage("... received " + this.floodNew + " msg");
        this.updateStatus();
        
        // display nodes online and search field
        this.frontend.showSearchField();
        this.showOnlineNodes();

        // highlight node passed in url
        let nodeId = window.location.hash.substring(1);
        if(nodeId) {
            this.setActiveNode(nodeId);
        }
    }

    initWebsocket() {
        this.socket = new WebSocket(
            ((window.location.protocol === "https:") ? "wss://" : "ws://") + this.url
        );
    
        this.socket.onerror = (e) => {
            this.setStatusMessage("WebSocket error observed. Please reload.");
            console.error("WebSocket error observed", e);
          };

        this.socket.onopen = () => {	
            this.setStatusMessage("WebSocket opened. Loading ... ");
            this.pingId = setInterval(() => {
                this.socket.send("_");
            }, 1000);
        };

        this.socket.onclose = () => {
            clearInterval(this.pingId)
            console.log("WebSocket connection was closed.")
        }

        this.socket.onmessage = (e) => {
            let type = e.data[0];
            let data = e.data.substr(1);
            let idA = data.substr(0, NODE_ID_LENGTH);
            let idB;
            
            if(!this.rendered) { this.floodNew++; }

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
                    this.ds.connectNodes(idA+idB, idA, idB);
                    break;
    
                case "c":
                    idB = data.substr(NODE_ID_LENGTH, NODE_ID_LENGTH);
                    console.log("disconnectNodes event:", idA, " - ", idB);
                    this.ds.disconnectNodes(idA+idB, idA, idB);
                    break;
            }
        }
    }
}


let app;
window.onload = () => {
    app = new Application(ANALYSIS_SERVER_URL);
    app.run()
}

function hasClass(elem, className) {
    return elem.classList.contains(className);
}

function parseColor(color) {
    var parsedColor = 0x009ee8ff;

    if (typeof color === 'string' && color) {
        if (color.length === 4) { // #rgb
            color = color.replace(/([^#])/g, '$1$1'); // duplicate each letter except first #.
        }
        if (color.length === 9) { // #rrggbbaa
            parsedColor = parseInt(color.substr(1), 16);
        } else if (color.length === 7) { // or #rrggbb.
            parsedColor = (parseInt(color.substr(1), 16) << 8) | 0xff;
        } else {
            throw 'Color expected in hex format with preceding "#". E.g. #00ff00. Got value: ' + color;
        }
    } else if (typeof color === 'number') {
        parsedColor = color;
    }

    return parsedColor;
}


/**
 * WebGL stuff 
 */

function WebGLCircle(size, color) {
    this.size = size;
    this.color = color;
}
// Next comes the hard part - implementation of API for custom shader
// program, used by webgl renderer:
function buildCircleNodeShader() {
    // For each primitive we need 4 attributes: x, y, color and size.
    var ATTRIBUTES_PER_PRIMITIVE = 4,
        nodesFS = [
            'precision mediump float;',
            'varying vec4 color;',
            'void main(void) {',
            '   if ((gl_PointCoord.x - 0.5) * (gl_PointCoord.x - 0.5) + (gl_PointCoord.y - 0.5) * (gl_PointCoord.y - 0.5) < 0.25) {',
            '     gl_FragColor = color;',
            '   } else {',
            '     gl_FragColor = vec4(0);',
            '   }',
            '}'].join('\n'),
        nodesVS = [
            'attribute vec2 a_vertexPos;',
            // Pack color and size into vector. First elemnt is color, second - size.
            // Since it's floating point we can only use 24 bit to pack colors...
            // thus alpha channel is dropped, and is always assumed to be 1.
            'attribute vec2 a_customAttributes;',
            'uniform vec2 u_screenSize;',
            'uniform mat4 u_transform;',
            'varying vec4 color;',
            'void main(void) {',
            '   gl_Position = u_transform * vec4(a_vertexPos/u_screenSize, 0, 1);',
            '   gl_PointSize = a_customAttributes[1] * u_transform[0][0];',
            '   float c = a_customAttributes[0];',
            '   color.b = mod(c, 256.0); c = floor(c/256.0);',
            '   color.g = mod(c, 256.0); c = floor(c/256.0);',
            '   color.r = mod(c, 256.0); c = floor(c/256.0); color /= 255.0;',
            '   color.a = 1.0;',
            '}'].join('\n');
    var program,
        gl,
        buffer,
        locations,
        utils,
        nodes = new Float32Array(64),
        nodesCount = 0,
        canvasWidth, canvasHeight, transform,
        isCanvasDirty;
    return {
        /**
         * Called by webgl renderer to load the shader into gl context.
         */
        load: function (glContext) {
            gl = glContext;
            webglUtils = Viva.Graph.webgl(glContext);
            program = webglUtils.createProgram(nodesVS, nodesFS);
            gl.useProgram(program);
            locations = webglUtils.getLocations(program, ['a_vertexPos', 'a_customAttributes', 'u_screenSize', 'u_transform']);
            gl.enableVertexAttribArray(locations.vertexPos);
            gl.enableVertexAttribArray(locations.customAttributes);
            buffer = gl.createBuffer();
        },
        /**
         * Called by webgl renderer to update node position in the buffer array
         *
         * @param nodeUI - data model for the rendered node (WebGLCircle in this case)
         * @param pos - {x, y} coordinates of the node.
         */
        position: function (nodeUI, pos) {
            var idx = nodeUI.id;
            nodes[idx * ATTRIBUTES_PER_PRIMITIVE] = pos.x;
            nodes[idx * ATTRIBUTES_PER_PRIMITIVE + 1] = -pos.y;
            nodes[idx * ATTRIBUTES_PER_PRIMITIVE + 2] = nodeUI.color;
            nodes[idx * ATTRIBUTES_PER_PRIMITIVE + 3] = nodeUI.size;
        },
        /**
         * Request from webgl renderer to actually draw our stuff into the
         * gl context. This is the core of our shader.
         */
        render: function () {
            gl.useProgram(program);
            gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
            gl.bufferData(gl.ARRAY_BUFFER, nodes, gl.DYNAMIC_DRAW);
            if (isCanvasDirty) {
                isCanvasDirty = false;
                gl.uniformMatrix4fv(locations.transform, false, transform);
                gl.uniform2f(locations.screenSize, canvasWidth, canvasHeight);
            }
            gl.vertexAttribPointer(locations.vertexPos, 2, gl.FLOAT, false, ATTRIBUTES_PER_PRIMITIVE * Float32Array.BYTES_PER_ELEMENT, 0);
            gl.vertexAttribPointer(locations.customAttributes, 2, gl.FLOAT, false, ATTRIBUTES_PER_PRIMITIVE * Float32Array.BYTES_PER_ELEMENT, 2 * 4);
            gl.drawArrays(gl.POINTS, 0, nodesCount);
        },
        /**
         * Called by webgl renderer when user scales/pans the canvas with nodes.
         */
        updateTransform: function (newTransform) {
            transform = newTransform;
            isCanvasDirty = true;
        },
        /**
         * Called by webgl renderer when user resizes the canvas with nodes.
         */
        updateSize: function (newCanvasWidth, newCanvasHeight) {
            canvasWidth = newCanvasWidth;
            canvasHeight = newCanvasHeight;
            isCanvasDirty = true;
        },
        /**
         * Called by webgl renderer to notify us that the new node was created in the graph
         */
        createNode: function (node) {
            nodes = webglUtils.extendArray(nodes, nodesCount, ATTRIBUTES_PER_PRIMITIVE);
            nodesCount += 1;
        },
        /**
         * Called by webgl renderer to notify us that the node was removed from the graph
         */
        removeNode: function (node) {
            if (nodesCount > 0) { nodesCount -= 1; }
            if (node.id < nodesCount && nodesCount > 0) {
                // we do not really delete anything from the buffer.
                // Instead we swap deleted node with the "last" node in the
                // buffer and decrease marker of the "last" node. Gives nice O(1)
                // performance, but make code slightly harder than it could be:
                webglUtils.copyArrayPart(nodes, node.id * ATTRIBUTES_PER_PRIMITIVE, nodesCount * ATTRIBUTES_PER_PRIMITIVE, ATTRIBUTES_PER_PRIMITIVE);
            }
        },
        /**
         * This method is called by webgl renderer when it changes parts of its
         * buffers. We don't use it here, but it's needed by API (see the comment
         * in the removeNode() method)
         */
        replaceProperties: function (replacedNode, newNode) { },
    };
}
