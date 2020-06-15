import {action, computed, observable, ObservableMap, ObservableSet} from "mobx";
import {connectWebSocket, registerHandler, WSMsgType} from "app/misc/WS";
import {default as Viva} from 'vivagraphjs';

export class AddNodeMessage {
    id: string;
}

export class RemoveNodeMessage {
    id: string;
}

export class ConnectNodesMessage {
    source: string;
    target: string
}

export class DisconnectNodesMessage {
    source: string;
    target: string
}

export class Neighbors {
    in: Set<string>;
    out: Set<string>;

    constructor() {
        this.in = new Set();
        this.out = new Set();
    }
}

const EDGE_COLOR_DEFAULT = "#ff7d6cff";
const EDGE_COLOR_HIDE = "#ff7d6c40";
const EDGE_COLOR_OUTGOING = "#336db5ff";
const EDGE_COLOR_INCOMING = "#1c8d7fff";
const VERTEX_COLOR_DEFAULT = "0xa8d0e6";
const VERTEX_COLOR_ACTIVE = "0xcb4b16";
const VERTEX_COLOR_IN_NEIGHBOR = "0x1c8d7f";
const VERTEX_COLOR_OUT_NEIGHBOR = "0x336db5";
const VERTEX_SIZE = 14;
const VERTEX_SIZE_ACTIVE = 24;
const VERTEX_SIZE_CONNECTED = 18;
const statusWebSocketPath = "/ws";

export const shortenedIDCharCount = 8;

export class AutopeeringStore {
    @observable nodes = new ObservableSet();
    @observable neighbors = new ObservableMap<string,Neighbors>();
    @observable connections = new ObservableSet();

    graphViewActive: boolean = false;
    @observable websocketConnected: boolean = false;

    // selecting a certain node
    @observable selectionActive: boolean = false;
    @observable selectedNode: string = null;
    @observable selectedNodeInNeighbors: Set<string> = null;
    @observable selectedNodeOutNeighbors: Set<string> = null;
    @observable previewNode: string = null;
    
    // search
    @observable search: string = "";

    // viva graph objects
    graph;
    graphics;
    renderer;

    constructor() {
        registerHandler(WSMsgType.AddNode, this.onAddNode);
        registerHandler(WSMsgType.RemoveNode, this.onRemoveNode);
        registerHandler(WSMsgType.ConnectNodes, this.onConnectNodes);
        registerHandler(WSMsgType.DisconnectNodes, this.onDisconnectNodes);
    }

    // connect to analysis server via websocket
    connect() {
        connectWebSocket(statusWebSocketPath,
            () => this.updateWebSocketConnected(true),
            () => this.updateWebSocketConnected(false),
            () => this.updateWebSocketConnected(false))
    }

    // derive the full node ID based on the shortened nodeID (first shortenedIDCharCount chars)
    getFullNodeID = (shortNodeID: string) => {
        for(let fullNodeID of this.nodes.values()){
            if (fullNodeID.startsWith(shortNodeID)) {
                return fullNodeID;
            }
        }
        return "";
    };

    @action
    updateWebSocketConnected = (connected: boolean) => this.websocketConnected = connected;

    // create a graph and fill it with data
    start = () => {
        this.graphViewActive = true;
        this.graph = Viva.Graph.graph();

        let graphics: any = Viva.Graph.View.webglGraphics();

        let layout = Viva.Graph.Layout.forceDirected(this.graph, {
            springLength: 30,
            springCoeff: 0.0001,
            dragCoeff: 0.02,
            stableThreshold: 0.15,
            gravity: -2,
            timeStep: 20,
            theta: 0.8,
        });
        graphics.link((link) => {
            return Viva.Graph.View.webglLine(EDGE_COLOR_DEFAULT);
        });
        graphics.setNodeProgram(buildCircleNodeShader());

        graphics.node((node) => {
            return new WebGLCircle(VERTEX_SIZE, VERTEX_COLOR_DEFAULT);
        });
        graphics.link(() => Viva.Graph.View.webglLine(EDGE_COLOR_DEFAULT));
        let ele = document.getElementById('visualizer');
        this.renderer = Viva.Graph.View.renderer(this.graph, {
            container: ele, graphics, layout, renderLinks: true,
        });

        let events = Viva.Graph.webglInputEvents(graphics, this.graph);

        events.click((node) => {
            this.handleNodeSelection(node.id);
        });

        events.mouseEnter((node) => {
            this.previewNode = node.id;
        });
        
        events.mouseLeave((node) => {
            this.previewNode = undefined;
        });


        this.graphics = graphics;
        this.renderer.run();
        // draw graph if we have data collected
        this.initialDrawGraph();
    }

    // fill graph with data we have previously collected
    initialDrawGraph = () => {
        this.nodes.forEach((node,key,map) => {
            this.drawNode(node);
        })
        this.neighbors.forEach((node,key,map) => {
            // Only do it for one type of neighbors, as it is duplicated
            node.out.forEach((outNeighborID) =>{
                this.graph.addLink(key, outNeighborID);
            })
        })
    }

    // dispose only graph, but keep the data
    stop = () => {
        this.graphViewActive = false;
        this.renderer.dispose();
        this.graph = null;
    }

    @action
    updateSearch = (searchNode: string) => {
        this.search = searchNode.trim();
    }

    // handlers for incoming ws messages //

    @action
    onAddNode = (msg: AddNodeMessage) => {
        if (this.nodes.has(msg.id)){
            console.log("Node %s already known.", msg.id);
            return;
        }
        this.nodes.add(msg.id);
        if (this.graphViewActive) {
            this.drawNode(msg.id);
        }
        console.log("Node %s added.", msg.id);

        // the more nodes we have, the more spacing we need
        if (this.nodes.size > 30) {
            this.renderer.getLayout().simulator.springLength(this.nodes.size);
        }
    }

    @action
    onRemoveNode = (msg: RemoveNodeMessage) => {
        if (!this.nodes.has(msg.id)) {
            console.log("Can't delete node %s, not in map.", msg.id);
            return
        }

        this.nodes.delete(msg.id);
        if (this.graphViewActive) {
            this.graph.removeNode(msg.id);
        }
        console.log("Removed node %s", msg.id)

        // the less nodes we have, the less spacing we need
        if (this.nodes.size >= 30) {
            this.renderer.getLayout().simulator.springLength(this.nodes.size);
        }
    }

    @action
    onConnectNodes = (msg: ConnectNodesMessage) => {
        if (!this.nodes.has(msg.source)) {
            console.log("Missing source node %s from node map.", msg.source);
            return;
        }
        if (!this.nodes.has(msg.target)) {
            console.log("Missing target node %s from node map.", msg.target);
            return;
        }

        // both are in the map, draw the connection on screen
        if (this.graphViewActive) {
            this.graph.addLink(msg.source, msg.target);
        }

        // update connections
        this.connections.add(msg.source + msg.target);

        // Update neighbors map
        if (this.neighbors.get(msg.source) === undefined) {
            let neighbors = new Neighbors();
            neighbors.out.add(msg.target);
            this.neighbors.set(msg.source, neighbors);
        } else {
            this.neighbors.get(msg.source).out.add(msg.target);
        }

        if (this.neighbors.get(msg.target) === undefined) {
            let neighbors = new Neighbors();
            neighbors.in.add(msg.source);
            this.neighbors.set(msg.target, neighbors);
        } else {
            this.neighbors.get(msg.target).in.add(msg.source);
        }

        console.log("Connected nodes %s -> %s", msg.source, msg.target);
    }

    @action
    onDisconnectNodes = (msg: DisconnectNodesMessage) => {
        if (this.graphViewActive){
            let existingLink = this.graph.getLink(msg.source, msg.target);
            if (!existingLink) {
                console.log("Link %s -> %s is missing from graph", msg.source, msg.target);
                return;
            }
            this.graph.removeLink(existingLink);
        }

        // update connections and neighbors
        this.connections.delete(msg.source + msg.target);
        this.neighbors.get(msg.source).out.delete(msg.target);
        this.neighbors.get(msg.target).in.delete(msg.source);

        console.log("Disconnected nodes %s -> %s",msg.source, msg.target)
    }

    // graph related updates //

    drawNode = (node: string) => {
        let existing = this.graph.getNode(node);

        if (existing) {
            return;
        } else {
            // add to graph structure
            this.graph.addNode(node);
        }
    }

    // updates color of a node (vertex) in the graph
    updateNodeUiColor = (node, color, size) => {
        let nodeUI = this.graphics.getNodeUI(node);
        if (nodeUI != undefined) {
            nodeUI.color = color;
            nodeUI.size = size;
        }
    }

    // updates color of a link (edge) in the graph
    updateLinkUiColor = (idA, idB, color) => {
        let con = this.graph.getLink(idA, idB);

        if(con != null) {
            let linkUI = this.graphics.getLinkUI(con.id);
            if (linkUI != undefined) {
                linkUI.color = parseColor(color);
            }
        }
    }

    // highlights selectedNode, its links and neighbors
    showHighlight = () => {
        if (!this.selectionActive) {return};

        this.graph.beginUpdate();

        this.graph.forEachLink((link) => {
            let linkUi = this.graphics.getLinkUI(link.id);
            linkUi.color = parseColor(EDGE_COLOR_HIDE);
        })

        // Highlight selected node
        this.updateNodeUiColor(this.selectedNode, VERTEX_COLOR_ACTIVE, VERTEX_SIZE_ACTIVE);
        this.selectedNodeInNeighbors.forEach((inNeighborID) => {
            this.updateNodeUiColor(inNeighborID, VERTEX_COLOR_IN_NEIGHBOR, VERTEX_SIZE_CONNECTED);
            this.updateLinkUiColor(inNeighborID, this.selectedNode, EDGE_COLOR_INCOMING);
        })
        this.selectedNodeOutNeighbors.forEach((outNeighborID) => {
            this.updateNodeUiColor(outNeighborID, VERTEX_COLOR_OUT_NEIGHBOR, VERTEX_SIZE_CONNECTED);
            this.updateLinkUiColor(this.selectedNode, outNeighborID, EDGE_COLOR_OUTGOING);
        })

        this.graph.endUpdate();
        this.renderer.rerender();
    }

    // disables highlighting of selectedNode, its links and neighbors
    resetPreviousColors = (skipAllLink: boolean = false, toLinkHide: boolean = false) => {
        if (!this.selectionActive) {return};
        this.graph.beginUpdate();

        let edgeColor = EDGE_COLOR_DEFAULT;

        if (toLinkHide) {
            edgeColor = EDGE_COLOR_HIDE;
        }

        // Remove highlighting of selected node
        this.updateNodeUiColor(this.selectedNode, VERTEX_COLOR_DEFAULT, VERTEX_SIZE);
        this.selectedNodeInNeighbors.forEach((inNeighborID) => {
            // Remove highlighting of neighbor
            this.updateNodeUiColor(inNeighborID, VERTEX_COLOR_DEFAULT, VERTEX_SIZE);
            // Remove highlighting of link
            this.updateLinkUiColor(inNeighborID, this.selectedNode, edgeColor);
        })
        this.selectedNodeOutNeighbors.forEach((outNeighborID) => {
            // Remove highlighting of neighbor
            this.updateNodeUiColor(outNeighborID, VERTEX_COLOR_DEFAULT, VERTEX_SIZE);
            // Remove highlighting of link
            this.updateLinkUiColor(this.selectedNode, outNeighborID, edgeColor);
        })

        if (!skipAllLink) {
            this.graph.forEachLink((link) => {
                let linkUi = this.graphics.getLinkUI(link.id);
                linkUi.color = parseColor(EDGE_COLOR_DEFAULT);
            })
        }

        this.graph.endUpdate();
        this.renderer.rerender();
    }

    // handlers for frontend events //

    // updates the currently selected node
    @action
    updateSelectedNode = (node: string) => {
        this.selectedNode = node;
        // get node incoming neighbors
        if (!this.nodes.has(this.selectedNode)) {
            console.log("Selected node not found (%s)", this.selectedNode);
        }
        const neighbors = this.neighbors.get(this.selectedNode);
        this.selectedNodeInNeighbors = neighbors ? neighbors.in : new Set();
        this.selectedNodeOutNeighbors =  neighbors ? neighbors.out : new Set();
        this.selectionActive = true;
        this.showHighlight();
    }

       // handles click on a node button
    @action
    handleNodeButtonOnClick = (e) => {
        // find node based on the first 8 characters
        let clickedNode = this.getFullNodeID(e.target.innerHTML)
        this.handleNodeSelection(clickedNode);
    }

    // checks whether selection is already active, then updates selected node
    @action
    handleNodeSelection = (clickedNode: string) => {
        if (this.selectionActive) {
            if (this.selectedNode === clickedNode) {
                // Disable selection on second click when clicked on the same node
                this.clearNodeSelection();
                return;
            } else {
                // we clicked on a different node
                // stop highlighting the other node if clicked
                // note that edge color defaults back to "hide"
                this.resetPreviousColors(true, true);
            }
        }
        this.updateSelectedNode(clickedNode);
    }

    // handles clearing the node selection
    @action
    clearNodeSelection = () => {
        this.resetPreviousColors();
        this.selectedNode = null;
        this.selectedNodeInNeighbors = null;
        this.selectedNodeOutNeighbors = null;
        this.selectionActive = false;
        return;
    }

    // computed values update frontend rendering //

    @computed
    get nodeListView(){
        let results;
        if (this.search.trim().length === 0) {
            results = this.nodes;
        } else {
            results = new Set();
            this.nodes.forEach((node) => {
                if (node.toLowerCase().indexOf(this.search.toLowerCase()) >= 0){
                    results.add(node);
                }
            })
        }
        let ids = [];

        results.forEach((nodeID) => {
            ids.push(nodeID);
        })
        return ids
    }

    @computed
    get inNeighborList(){
        return Array.from(this.selectedNodeInNeighbors);
    }

    @computed
    get outNeighborList(){
        return Array.from(this.selectedNodeOutNeighbors);
    }

}

export default AutopeeringStore;

// vivagraph related utility functions //

function parseColor(color): any {
    let parsedColor = 0x009ee8ff;

    if (typeof color === 'number') {
        return color;
    }

    if (typeof color === 'string' && color) {
        if (color.length === 4) {
            // #rgb, duplicate each letter except first #.
            color = color.replace(/([^#])/g, '$1$1');
        }
        if (color.length === 9) {
            // #rrggbbaa
            parsedColor = parseInt(color.substr(1), 16);
        } else if (color.length === 7) {
            // or #rrggbb.
            parsedColor = (parseInt(color.substr(1), 16) << 8) | 0xff;
        } else {
            throw 'Color expected in hex format with preceding "#". E.g. #00ff00. Got value: ' + color;
        }
    }

    return parsedColor;
}

// WebGL stuff //

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
        webglUtils,
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
