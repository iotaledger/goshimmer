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
import { Neighbors } from "../models/Neighbors";
import { manaStore } from "../../main";
import tinycolor from "tinycolor2";

const EDGE_COLOR_DEFAULT = "#ff7d6cff";
const EDGE_COLOR_HIDE = "#ff7d6c40";
const EDGE_COLOR_OUTGOING = "#336db5ff";
const EDGE_COLOR_INCOMING = "#1c8d7fff";
// default vertex color is #00ffff
const VERTEX_COLOR_DEFAULT = "0x" + tinycolor("hsl(180, 100%, 50%)").toHex();
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
    public selectedNetworkVersion: string = "";

    @observable
    public userSelectedNetworkVersion: string = "";

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
    public manaColoringActive: boolean;

    @observable
    public readonly versions: ObservableSet = new ObservableSet()

    @observable
    public readonly nodes: ObservableMap<string, ObservableSet<string>> = new ObservableMap<string, ObservableSet<string>>();

    // keeps a log of what color a node should have, updated based on mana
    // maps shortNodeID -> color string
    @observable
    public nodeManaColors: ObservableMap<string, string> = new ObservableMap<string, string>();

    @observable
    public readonly connections: ObservableMap<string, ObservableSet<string>> = new ObservableMap<string, ObservableSet<string>>();

    @observable
    private readonly neighbors: ObservableMap<string, ObservableMap<string, INeighbors>> = new ObservableMap<string, ObservableMap<string, INeighbors>>();

    @observable
    private selectionActive: boolean = false;

    private graph?: Viva.Graph.IGraph;

    private graphics: Viva.Graph.View.IWebGLGraphics;

    private renderer: Viva.Graph.View.IRenderer;

    private colorBrokerID;

    constructor() {
        this.manaColoringActive = false;
        registerHandler(WSMsgType.addNode, msg => this.onAddNode(msg));
        registerHandler(WSMsgType.removeNode, msg => this.onRemoveNode(msg));
        registerHandler(WSMsgType.connectNodes, msg => this.onConnectNodes(msg));
        registerHandler(WSMsgType.disconnectNodes, msg => this.onDisconnectNodes(msg));
        registerHandler(WSMsgType.MsgManaDashboardAddress, msg => this.setManaDashboardAddress(msg))
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
    public handleVersionSelection = (userSelectedVersion: string) => {
        this.userSelectedNetworkVersion = userSelectedVersion;
        if (this.selectedNetworkVersion !== userSelectedVersion) {
            // we switch network, should redraw the graph.
            this.clearNodeSelection();
            this.selectedNetworkVersion = userSelectedVersion;
            this.stop();
            this.start();
        }
    }

    @action
    public handleAutoVersionSelection = (autoSelectedVersion: string) => {
        if (this.selectedNetworkVersion !== autoSelectedVersion) {
            this.userSelectedNetworkVersion = "";
            // we switch network, should redraw the graph.
            // no need to reset colors as the graph will be disposed anyway
            this.clearNodeSelection(false);
            this.selectedNetworkVersion = autoSelectedVersion;
            if (this.graphics) {
                this.stop();
                this.start();
            }
        }
    }

    @action
    handleManaColoringChange = (checked: boolean) => {
        if (checked) {
            if (this.manaColoringActive) {
                // button says turn it on. but it is already on
                return;
            } else {
                // button says turn it on, it was off
                this.manaColoringActive = checked;
                this.updateNodeColoring();
            }
        } else {
            if (this.manaColoringActive) {
                // button says turn it off and it is on
                this.manaColoringActive = checked;
                this.disableNodeColoring();
            } else {
                // button says turn it off, it was off
                return;
            }
        }
    }

    @action
    private autoSelectNetwork = () => {
        if (this.versions.size === 0) {
            return "";
        }
        let versionsArray = Array.from(this.versions);
        versionsArray.sort((a, b) => {
            return b.localeCompare(a);
        })
        return versionsArray[0];

    }

    @action
    private setManaDashboardAddress(address: string): void {
        manaStore.setManaDashboardAddress(address)
    }

    @action
    private onAddNode(msg: IAddNodeMessage): void {
        if (!this.versions.has(msg.networkVersion)) {
            this.versions.add(msg.networkVersion);
            if (this.userSelectedNetworkVersion === "") {
                // usert hasn't specified a network version yet, we choose the newest
                // otherwise we display wahtever the user selected
                this.handleAutoVersionSelection(this.autoSelectNetwork());
            }
        }
        // when we see the network for the first time
        if (this.nodes.get(msg.networkVersion) === undefined) {
            this.nodes.set(msg.networkVersion, new ObservableSet<string>());
        }

        let nodeSet = this.nodes.get(msg.networkVersion);
        // @ts-ignore
        if (nodeSet.has(msg.id)) {
            console.log("Node %s already known.", msg.id);
            return;
        }
        // @ts-ignore
        nodeSet.add(msg.id);
        console.log("Node %s added, network: %s", msg.id, msg.networkVersion);
        // only update visuals when the current network is displayed
        if (this.selectedNetworkVersion === msg.networkVersion) {
            if (this.graph) {
                this.drawNode(msg.id);
            }

            // the more nodes we have, the more spacing we need
            // @ts-ignore
            if (nodeSet.size > 30) {
                // @ts-ignore
                this.renderer.getLayout().simulator.springLength(nodeSet.size);
            }
        }
    }

    @action
    private onRemoveNode(msg: IRemoveNodeMessage): void {
        if (this.nodes.get(msg.networkVersion) === undefined) {
            // nowhere to remove it from
            return;
        }
        let nodeSet = this.nodes.get(msg.networkVersion);
        // @ts-ignore
        if (!nodeSet.has(msg.id)) {
            console.log("Can't delete node %s, not in map.", msg.id);
            return
        }

        if (this.selectedNetworkVersion === msg.networkVersion) {
            if (this.graph) {
                this.graph.removeNode(msg.id);
            }

            // the less nodes we have, the less spacing we need
            // @ts-ignore
            if (nodeSet.size >= 30) {
                // @ts-ignore
                this.renderer.getLayout().simulator.springLength(nodeSet.size);
            }
        }
        // @ts-ignore
        nodeSet.delete(msg.id);
        // @ts-ignore
        if (nodeSet.size === 0) {
            // this was the last node for this network version
            this.nodes.delete(msg.networkVersion);
            this.neighbors.delete(msg.networkVersion);
            this.connections.delete(msg.networkVersion);
            this.versions.delete(msg.networkVersion);
            console.log("Removed all data for network %s", msg.networkVersion);

            if (this.selectedNetworkVersion === msg.networkVersion) {
                // we were looking at this network, but it gets deleted.
                // auto select
                this.handleAutoVersionSelection(this.autoSelectNetwork());
            }
        }
        console.log("Removed node %s, network: %s", msg.id, msg.networkVersion);
    }

    @action
    private onConnectNodes(msg: IConnectNodesMessage): void {
        if (this.nodes.get(msg.networkVersion) === undefined) {
            // we haven't seen this network before. trigger addnode (creates the set)
            this.onAddNode({ networkVersion: msg.networkVersion, id: msg.source });
            this.onAddNode({ networkVersion: msg.networkVersion, id: msg.target });
        }
        let nodeSet = this.nodes.get(msg.networkVersion);
        // @ts-ignore
        if (!nodeSet.has(msg.source)) {
            console.log("Missing source node %s from node map.", msg.source);
            return;
        }
        // @ts-ignore
        if (!nodeSet.has(msg.target)) {
            console.log("Missing target node %s from node map.", msg.target);
            return;
        }

        // both are in the map, draw the connection on screen
        if (this.graph && this.selectedNetworkVersion === msg.networkVersion) {
            this.graph.addLink(msg.source, msg.target);
        }

        // first time we see connectNodes for this network
        if (this.connections.get(msg.networkVersion) === undefined) {
            this.connections.set(msg.networkVersion, new ObservableSet<string>());
        }
        // update connections
        // @ts-ignore
        this.connections.get(msg.networkVersion).add(msg.source + msg.target);

        // first time we see connectNodes for this network
        if (this.neighbors.get(msg.networkVersion) === undefined) {
            this.neighbors.set(msg.networkVersion, new ObservableMap<string, INeighbors>());
        }

        let neighborMap = this.neighbors.get(msg.networkVersion);

        // Update neighbors map
        // @ts-ignore
        if (neighborMap.get(msg.source) === undefined) {
            let neighbors = new Neighbors();
            neighbors.out.add(msg.target);
            // @ts-ignore
            neighborMap.set(msg.source, neighbors);
        } else {
            // @ts-ignore
            neighborMap.get(msg.source).out.add(msg.target);
        }

        // @ts-ignore
        if (neighborMap.get(msg.target) === undefined) {
            let neighbors = new Neighbors();
            neighbors.in.add(msg.source);
            // @ts-ignore
            neighborMap.set(msg.target, neighbors);
        } else {
            // @ts-ignore
            neighborMap.get(msg.target).in.add(msg.source);
        }

        console.log("Connected nodes %s -> %s, network: %s", msg.source, msg.target, msg.networkVersion);
    }

    @action
    private onDisconnectNodes(msg: IDisconnectNodesMessage): void {
        if (this.graph && this.selectedNetworkVersion === msg.networkVersion) {
            let existingLink = this.graph.getLink(msg.source, msg.target);
            if (!existingLink) {
                console.log("Link %s -> %s is missing from graph", msg.source, msg.target);
                return;
            }
            this.graph.removeLink(existingLink);
        }

        // update connections and neighbors
        if (this.connections.get(msg.networkVersion) === undefined) {
            console.log("Can't find connections set for %s", msg.networkVersion)
        } else {
            // @ts-ignore
            this.connections.get(msg.networkVersion).delete(msg.source + msg.target);
        }

        if (this.neighbors.get(msg.networkVersion) === undefined) {
            console.log("Can't find neighbors map for %s", msg.networkVersion)
        } else {
            // @ts-ignore
            this.neighbors.get(msg.networkVersion).get(msg.source).out.delete(msg.target);
            // @ts-ignore
            this.neighbors.get(msg.networkVersion).get(msg.target).in.delete(msg.source);

        }
        console.log("Disconnected nodes %s -> %s, network: %s", msg.source, msg.target, msg.networkVersion);
    }


    // updates the currently selected node
    @action
    private updateSelectedNode(node: string): void {
        this.selectedNode = node;

        // get node incoming neighbors
        // @ts-ignore
        if (!this.nodes.get(this.selectedNetworkVersion).has(this.selectedNode)) {
            console.log("Selected node not found (%s)", this.selectedNode);
            return;
        }
        // @ts-ignore
        const neighbors = this.neighbors.get(this.selectedNetworkVersion).get(this.selectedNode);
        this.selectedNodeInNeighbors = neighbors ? neighbors.in : new Set();
        this.selectedNodeOutNeighbors = neighbors ? neighbors.out : new Set();
        this.selectionActive = true;
        this.showHighlight();
    }

    // handles clearing the node selection
    @action
    private clearNodeSelection(resetPrevColors: boolean = true): void {
        if (resetPrevColors) {
            this.resetPreviousColors();
        }
        this.selectedNode = undefined;
        this.selectedNodeInNeighbors = undefined;
        this.selectedNodeOutNeighbors = undefined;
        this.selectionActive = false;
    }

    reconnect() {
        this.updateWebSocketConnected(false)
        setTimeout(() => {
            this.connect();
        }, 5000);
    }

    // connect to analysis server via websocket
    public connect(): void {
        connectWebSocket(statusWebSocketPath,
            () => this.updateWebSocketConnected(true),
            () => this.reconnect(),
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
        this.initialDrawGraph(this.selectedNetworkVersion);

        // color nodes based on mana
        this.updateNodeColoring();
        this.colorBrokerID = setInterval(() => {
            this.updateNodeColoring();
        }, 5000)
    }

    // dispose only graph, but keep the data
    public stop(): void {
        clearInterval(this.colorBrokerID);
        this.renderer.dispose();
        this.graph = undefined;
    }

    @computed
    public get nodeListView(): string[] {
        let nodeSet = this.nodes.get(this.selectedNetworkVersion);
        if (nodeSet === undefined) {
            return [];
        }
        let results;
        if (this.search.trim().length === 0) {
            results = nodeSet;
        } else {
            results = new Set();
            // @ts-ignore
            nodeSet.forEach((node) => {
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

    @computed
    public get networkVersionList(): string[] {
        return Array.from(this.versions).sort((a, b) => {
            return a.localeCompare(b);
        });
    }

    @computed
    public get AvgNumNeighbors(): string {
        let nodeSet = this.nodes.get(this.selectedNetworkVersion);
        let connectionSet = this.connections.get(this.selectedNetworkVersion);
        if (nodeSet === undefined || connectionSet === undefined) {
            return "0"
        }
        // @ts-ignore
        return nodeSet && nodeSet.size > 0 ?
            // @ts-ignore
            (2 * connectionSet.size / nodeSet.size).toPrecision(2).toString()
            : "0"
    }

    @computed
    get NodesOnline() {
        let nodeSet = this.nodes.get(this.selectedNetworkVersion);
        if (nodeSet === undefined) {
            return "0"
        }

        // @ts-ignore
        return nodeSet.size.toString()
    }


    // fill graph with data we have previously collected
    private initialDrawGraph(version: string): void {
        if (this.graph) {
            if (this.nodes.get(version) !== undefined) {
                // @ts-ignore
                this.nodes.get(version).forEach((node, key, map) => {
                    this.drawNode(node);
                })
            }
            if (this.neighbors.get(version) !== undefined) {
                // @ts-ignore
                this.neighbors.get(version).forEach((node, key, map) => {
                    // Only do it for one type of neighbors, as it is duplicated
                    node.out.forEach((outNeighborID) => {
                        // @ts-ignore
                        this.graph.addLink(key, outNeighborID);
                    });
                });
            }
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
        this.renderer.rerender();
    }

    @action
    updateNodeColoring = () => {
        if (!this.manaColoringActive || this.selectionActive || !this.graphics) {
            return;
        }
        this.nodeManaColors.forEach((color, nodeID) => {
            let nodeUI = this.graphics.getNodeUI(nodeID)
            if (!nodeUI) {
                // node is not part of the current view
                return;
            }
            nodeUI.color = color;
        })
        this.renderer.rerender();
    }

    @action
    disableNodeColoring = () => {
        if (!this.graphics) {
            return;
        }
        this.nodeManaColors.forEach((color, nodeID) => {
            let nodeUI = this.graphics.getNodeUI(nodeID)
            if (!nodeUI) {
                // node is not part of the current view
                return;
            }
            nodeUI.color = VERTEX_COLOR_DEFAULT;
        })
        this.renderer.rerender();
    }

    @action
    public updateColorBasedOnMana(nodeId: string, mana: number): void {
        // pick a color based on mana value
        let colorMana = this.getColorFromMana(mana);
        this.nodeManaColors.set(nodeId, colorMana)
    }

    getColorFromMana = (mana: number) => {
        let logMana = 0
        if (mana >= 1.0) {
            logMana = Math.log(mana)
        }
        let logMaxMana = Math.log(1000000000000000.0)
        let hue = 180.0 * (1.0 - logMana / logMaxMana);
        let c = tinycolor(({ h: hue, s: 1, l: .5 }));
        return "0x" + c.toHex();
    }

    // updates color of a link (edge) in the graph
    private updateLinkUiColor(idA: string, idB: string, color: string): void {
        if (this.graph) {
            const con = this.graph.getLink(idA, idB);

            if (con) {
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
        if (!this.selectionActive || !this.graph) {
            return;
        }

        this.graph.beginUpdate();

        let edgeColor = EDGE_COLOR_DEFAULT;

        if (toLinkHide) {
            edgeColor = EDGE_COLOR_HIDE;
        }

        // Remove highlighting of selected node
        if (this.selectedNode) {
            let prevColor = this.nodeManaColors.get(this.selectedNode);
            if (!prevColor || !this.manaColoringActive) {
                prevColor = VERTEX_COLOR_DEFAULT;
            }
            this.updateNodeUiColor(this.selectedNode, prevColor, VERTEX_SIZE);

            if (this.selectedNodeInNeighbors) {
                for (const inNeighborID of this.selectedNodeInNeighbors) {
                    let prevColor = this.nodeManaColors.get(inNeighborID);
                    if (!prevColor || !this.manaColoringActive) {
                        prevColor = VERTEX_COLOR_DEFAULT;
                    }
                    // Remove highlighting of neighbor
                    this.updateNodeUiColor(inNeighborID, prevColor, VERTEX_SIZE);
                    // Remove highlighting of link
                    this.updateLinkUiColor(inNeighborID, this.selectedNode, edgeColor);
                }
            }
            if (this.selectedNodeOutNeighbors) {
                for (const outNeighborID of this.selectedNodeOutNeighbors) {
                    let prevColor = this.nodeManaColors.get(outNeighborID);
                    if (!prevColor || !this.manaColoringActive) {
                        prevColor = VERTEX_COLOR_DEFAULT;
                    }
                    // Remove highlighting of neighbor
                    this.updateNodeUiColor(outNeighborID, prevColor, VERTEX_SIZE);
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
        this.updateNodeColoring();
    }
}
