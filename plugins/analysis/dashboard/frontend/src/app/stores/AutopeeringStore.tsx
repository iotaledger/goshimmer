import { action, computed, observable, ObservableMap, ObservableSet } from "mobx";
import Viva from "vivagraphjs";
import { INeighbors } from "../models/INeigbours";
import { IAddNodeMessage } from "../models/messages/IAddNodeMessage";
import { IConnectNodesMessage } from "../models/messages/IConnectNodesMessage";
import { IDisconnectNodesMessage } from "../models/messages/IDisconnectNodesMessage";
import { IRemoveNodeMessage } from "../models/messages/IRemoveNodeMessage";
import { WSMsgType } from "../models/ws/wsMsgType";
import { connectWebSocket, registerHandler } from "../services/WS";
import { buildCircleNodeShader } from "../utils/circleNodeShader";
import { parseColor } from "../utils/colorHelper";

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
    @observable
    public websocketConnected: boolean = false;

    @observable
    public search: string = "";

    @observable
    public previewNode?: string;

    @observable
    public selectedNode?: string;

    @observable
    public selectedNodeInNeighbors?: Set<string>;

    @observable
    public selectedNodeOutNeighbors?: Set<string>;

    @observable
    public readonly nodes: ObservableSet<string> = new ObservableSet<string>();

    @observable
    public readonly connections: ObservableSet = new ObservableSet();

    @observable
    private readonly neighbors: ObservableMap<string, INeighbors> = new ObservableMap<string, INeighbors>();

    @observable
    private selectionActive: boolean = false;

    private graph?: Viva.Graph.IGraph;

    private graphics: Viva.Graph.View.IWebGLGraphics;

    private renderer: Viva.Graph.View.IRenderer;

    constructor() {
        registerHandler(WSMsgType.addNode, msg => this.onAddNode(msg));
        registerHandler(WSMsgType.removeNode, msg => this.onRemoveNode(msg));
        registerHandler(WSMsgType.connectNodes, msg => this.onConnectNodes(msg));
        registerHandler(WSMsgType.disconnectNodes, msg => this.onDisconnectNodes(msg));
    }

    // checks whether selection is already active, then updates selected node
    @action
    public handleNodeSelection(clickedNode: string): void {
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


    @action
    public updateWebSocketConnected(connected: boolean): void {
        this.websocketConnected = connected;
    }

    @action
    public updateSearch(searchNode: string): void {
        this.search = searchNode.trim();
    }

    @action
    private onAddNode(msg: IAddNodeMessage): void {
        if (this.nodes.has(msg.id)) {
            console.log("Node %s already known.", msg.id);
            return;
        }
        this.nodes.add(msg.id);
        if (this.graph) {
            this.drawNode(msg.id);
        }
        console.log("Node %s added.", msg.id);

        // the more nodes we have, the more spacing we need
        if (this.nodes.size > 30) {
            this.renderer.getLayout().simulator.springLength(this.nodes.size);
        }
    }

    @action
    private onRemoveNode(msg: IRemoveNodeMessage): void {
        if (!this.nodes.has(msg.id)) {
            console.log("Can't delete node %s, not in map.", msg.id);
            return;
        }

        this.nodes.delete(msg.id);
        if (this.graph) {
            this.graph.removeNode(msg.id);
        }
        console.log("Removed node %s", msg.id);

        // the less nodes we have, the less spacing we need
        if (this.nodes.size >= 30) {
            this.renderer.getLayout().simulator.springLength(this.nodes.size);
        }
    }

    @action
    private onConnectNodes(msg: IConnectNodesMessage): void {
        if (!this.nodes.has(msg.source)) {
            console.log("Missing source node %s from node map.", msg.source);
            return;
        }
        if (!this.nodes.has(msg.target)) {
            console.log("Missing target node %s from node map.", msg.target);
            return;
        }

        // both are in the map, draw the connection on screen
        if (this.graph) {
            this.graph.addLink(msg.source, msg.target);
        }

        // update connections
        this.connections.add(msg.source + msg.target);

        // Update neighbors map
        if (this.neighbors.get(msg.source) === undefined) {
            const neighbors: INeighbors = {
                in: new Set<string>(),
                out: new Set<string>()
            };
            neighbors.out.add(msg.target);
            this.neighbors.set(msg.source, neighbors);
        } else {
            const neighbour = this.neighbors.get(msg.source);

            if (neighbour) {
                neighbour.out.add(msg.target);
            }
        }

        if (this.neighbors.get(msg.target) === undefined) {
            const neighbors: INeighbors = {
                in: new Set<string>(),
                out: new Set<string>()
            };
            neighbors.in.add(msg.source);
            this.neighbors.set(msg.target, neighbors);
        } else {
            const neighbour = this.neighbors.get(msg.target);

            if (neighbour) {
                neighbour.in.add(msg.source);
            }
        }

        console.log("Connected nodes %s -> %s", msg.source, msg.target);
    }

    @action
    private onDisconnectNodes(msg: IDisconnectNodesMessage): void {
        if (this.graph) {
            const existingLink = this.graph.getLink(msg.source, msg.target);
            if (!existingLink) {
                console.log("Link %s -> %s is missing from graph", msg.source, msg.target);
                return;
            }
            this.graph.removeLink(existingLink);
        }

        // update connections and neighbors
        this.connections.delete(msg.source + msg.target);

        const source = this.neighbors.get(msg.source);
        if (source) {
            source.out.delete(msg.target);
        }
        const target = this.neighbors.get(msg.target);
        if (target) {
            target.in.delete(msg.source);
        }

        console.log("Disconnected nodes %s -> %s", msg.source, msg.target);
    }

        
    // updates the currently selected node
    @action
    private updateSelectedNode(node: string): void {
        this.selectedNode = node;

        // get node incoming neighbors
        if (!this.nodes.has(this.selectedNode)) {
            console.log("Selected node not found (%s)", this.selectedNode);
        }
        const neighbors = this.neighbors.get(this.selectedNode);
        this.selectedNodeInNeighbors = neighbors ? neighbors.in : new Set();
        this.selectedNodeOutNeighbors = neighbors ? neighbors.out : new Set();
        this.selectionActive = true;
        this.showHighlight();
    }

    // handles clearing the node selection
    @action
    private clearNodeSelection(): void {
        this.resetPreviousColors();
        this.selectedNode = undefined;
        this.selectedNodeInNeighbors = undefined;
        this.selectedNodeOutNeighbors = undefined;
        this.selectionActive = false;
    }
    
    // connect to analysis server via websocket
    public connect(): void {
        connectWebSocket(statusWebSocketPath,
            () => this.updateWebSocketConnected(true),
            () => this.updateWebSocketConnected(false),
            () => this.updateWebSocketConnected(false));
    }

    // create a graph and fill it with data
    public start(): void {
        this.graph = Viva.Graph.graph();

        const graphics = Viva.Graph.View.webglGraphics();

        const layout = Viva.Graph.Layout.forceDirected(this.graph, {
            springLength: 30,
            springCoeff: 0.0001,
            dragCoeff: 0.02,
            stableThreshold: 0.15,
            gravity: -2,
            timeStep: 20,
            theta: 0.8
        });
        graphics.link(() => {
            return Viva.Graph.View.webglLine(EDGE_COLOR_DEFAULT);
        });
        graphics.setNodeProgram(buildCircleNodeShader());

        graphics.node(() => {
            return {
                size: VERTEX_SIZE,
                color: VERTEX_COLOR_DEFAULT
            };
        });
        graphics.link(() => Viva.Graph.View.webglLine(EDGE_COLOR_DEFAULT));
        const ele = document.getElementById("visualizer");
        this.renderer = Viva.Graph.View.renderer(this.graph, {
            container: ele, graphics, layout, renderLinks: true
        });

        const events = Viva.Graph.webglInputEvents(graphics, this.graph);

        events.click((node) => {
            this.handleNodeSelection(node.id);
        });

        events.mouseEnter((node) => {
            this.previewNode = node.id;
        });

        events.mouseLeave(() => {
            this.previewNode = undefined;
        });


        this.graphics = graphics;
        this.renderer.run();
        // draw graph if we have data collected
        this.initialDrawGraph();
    }

    // dispose only graph, but keep the data
    public stop(): void {
        this.renderer.dispose();
        this.graph = undefined;
    }

    @computed
    public get nodeListView(): string[] {
        let results: Set<string>;
        if (this.search.trim().length === 0) {
            results = this.nodes;
        } else {
            results = new Set<string>();
            this.nodes.forEach((node) => {
                if (node.toLowerCase().includes(this.search.toLowerCase())) {
                    results.add(node);
                }
            });
        }
        const ids: string[] = [];
        results.forEach((nodeID) => {
            ids.push(nodeID);
        });
        return ids;
    }

    @computed
    public get inNeighborList(): string[] {
        return this.selectedNodeInNeighbors ? Array.from(this.selectedNodeInNeighbors) : [];
    }

    @computed
    public get outNeighborList(): string[] {
        return this.selectedNodeOutNeighbors ? Array.from(this.selectedNodeOutNeighbors) : [];
    }

    // fill graph with data we have previously collected
    private initialDrawGraph(): void {
        if (this.graph) {
            this.nodes.forEach((node) => {
                this.drawNode(node);
            });
            this.neighbors.forEach((node, key) => {
                // Only do it for one type of neighbors, as it is duplicated
                node.out.forEach((outNeighborID) => {
                    if (this.graph) {
                        this.graph.addLink(key, outNeighborID);
                    }
                });
            });
        }
    }

    // graph related updates //
    private drawNode(node: string): void {
        if (this.graph) {
            const existing = this.graph.getNode(node);

            if (!existing) {
                // add to graph structure
                this.graph.addNode(node);
            }
        }
    }

    // updates color of a node (vertex) in the graph
    private updateNodeUiColor(node: string, color: string, size: number): void {
        const nodeUI = this.graphics.getNodeUI(node);
        if (nodeUI !== undefined) {
            nodeUI.color = color;
            nodeUI.size = size;
        }
    }

    // updates color of a link (edge) in the graph
    private updateLinkUiColor(idA: string, idB: string, color: string): void {
        if (this.graph) {
            const con = this.graph.getLink(idA, idB);

            if (con !== undefined) {
                const linkUI = this.graphics.getLinkUI(con.id);
                if (linkUI !== undefined) {
                    linkUI.color = parseColor(color);
                }
            }
        }
    }

    // highlights selectedNode, its links and neighbors
    private showHighlight(): void {
        if (!this.selectionActive) {
            return;
        }

        if (!this.graph) {
            return;
        }

        this.graph.beginUpdate();

        this.graph.forEachLink((link) => {
            const linkUi = this.graphics.getLinkUI(link.id);
            if (linkUi) {
                linkUi.color = parseColor(EDGE_COLOR_HIDE);
            }
        });

        // Highlight selected node
        if (this.selectedNode) {
            this.updateNodeUiColor(this.selectedNode, VERTEX_COLOR_ACTIVE, VERTEX_SIZE_ACTIVE);

            if (this.selectedNodeInNeighbors) {
                for (const inNeighborID of this.selectedNodeInNeighbors) {
                    this.updateNodeUiColor(inNeighborID, VERTEX_COLOR_IN_NEIGHBOR, VERTEX_SIZE_CONNECTED);
                    this.updateLinkUiColor(inNeighborID, this.selectedNode, EDGE_COLOR_INCOMING);
                }
            }
            if (this.selectedNodeOutNeighbors) {
                for (const outNeighborID of this.selectedNodeOutNeighbors) {
                    this.updateNodeUiColor(outNeighborID, VERTEX_COLOR_OUT_NEIGHBOR, VERTEX_SIZE_CONNECTED);
                    this.updateLinkUiColor(this.selectedNode, outNeighborID, EDGE_COLOR_OUTGOING);
                }
            }
        }

        this.graph.endUpdate();
        this.renderer.rerender();
    }

    // disables highlighting of selectedNode, its links and neighbors
    private resetPreviousColors(skipAllLink: boolean = false, toLinkHide: boolean = false): void {
        if (!this.selectionActive) {
            return;
        }

        if (!this.graph) {
            return;
        }

        this.graph.beginUpdate();

        let edgeColor = EDGE_COLOR_DEFAULT;

        if (toLinkHide) {
            edgeColor = EDGE_COLOR_HIDE;
        }

        // Remove highlighting of selected node
        if (this.selectedNode) {
            this.updateNodeUiColor(this.selectedNode, VERTEX_COLOR_DEFAULT, VERTEX_SIZE);

            if (this.selectedNodeInNeighbors) {
                for (const inNeighborID of this.selectedNodeInNeighbors) {
                    // Remove highlighting of neighbor
                    this.updateNodeUiColor(inNeighborID, VERTEX_COLOR_DEFAULT, VERTEX_SIZE);
                    // Remove highlighting of link
                    this.updateLinkUiColor(inNeighborID, this.selectedNode, edgeColor);
                }
            }
            if (this.selectedNodeOutNeighbors) {
                for (const outNeighborID of this.selectedNodeOutNeighbors) {
                    // Remove highlighting of neighbor
                    this.updateNodeUiColor(outNeighborID, VERTEX_COLOR_DEFAULT, VERTEX_SIZE);
                    // Remove highlighting of link
                    this.updateLinkUiColor(this.selectedNode, outNeighborID, edgeColor);
                }
            }
        }

        if (!skipAllLink) {
            this.graph.forEachLink((link) => {
                const linkUi = this.graphics.getLinkUI(link.id);
                if (linkUi) {
                    linkUi.color = parseColor(EDGE_COLOR_DEFAULT);
                }
            });
        }

        this.graph.endUpdate();
        this.renderer.rerender();
    }
}
