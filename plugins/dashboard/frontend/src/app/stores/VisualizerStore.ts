import { action, observable, ObservableMap } from 'mobx';
import { registerHandler, WSMsgType } from "app/misc/WS";
import { RouterStore } from "mobx-react-router";
import { default as Viva } from 'vivagraphjs';

export class Vertex {
    id: string;
    parentIDsByType: Map<String, Array<string>>;
    is_tip: boolean;
    is_finalized: boolean;
    is_tx: boolean;
}

export class TipInfo {
    id: string;
    is_tip: boolean;
}

class history {
    vertices: Array<Vertex>;
}

const vertexSize = 20;

export class VisualizerStore {
    @observable vertices = new ObservableMap<string, Vertex>();
    @observable verticesLimit = 1500;
    @observable finalized_count = 0;
    @observable tips_count = 0;
    verticesIncomingOrder = [];
    draw: boolean = false;
    routerStore: RouterStore;

    // the currently selected vertex via hover
    @observable selected: Vertex;
    selected_via_click: boolean = false;
    selected_origin_color: number = 0;

    // search
    @observable search: string = "";

    // viva graph objs
    graph;
    graphics;
    renderer;
    @observable paused: boolean = false;

    constructor(routerStore: RouterStore) {
        this.routerStore = routerStore;
        this.fetchHistory();
        registerHandler(WSMsgType.Vertex, this.addVertex);
        registerHandler(WSMsgType.TipInfo, this.addTipInfo);
    }

    fetchHistory = async () => {
        try {
            let res = await fetch(`/api/visualizer/history`);
            let history: history = await res.json();
            history.vertices.forEach(v => {
                this.addVertex(v);
            });
        } catch (err) {
            console.log("Fail to fetch history in visualizer", err);
        }
        return
    }

    @action
    updateSearch = (search: string) => {
        this.search = search.trim();
    }

    @action
    searchAndHighlight = () => {
        this.clearSelected();
        if (!this.search) return;
        let iter: IterableIterator<string> = this.vertices.keys();
        let found = null;
        for (const key of iter) {
            if (key.indexOf(this.search) >= 0) {
                found = key;
                break;
            }
        }
        if (!found) return;
        this.updateSelected(this.vertices.get(found), false);
    }

    @action
    pauseResume = () => {
        if (this.paused) {
            this.renderer.resume();
            this.paused = false;
            return;
        }
        this.renderer.pause();
        this.paused = true;
    }

    @action
    updateVerticesLimit = (num: number) => {
        this.verticesLimit = num;
    }

    @action
    addVertex = (vert: Vertex) => {
        let existing = this.vertices.get(vert.id);
        if (existing) {
            if (!existing.is_finalized && vert.is_finalized) {
                existing.is_finalized = true;
                this.finalized_count++;
            }
            // update parent1 and parent2 ids since we might be dealing
            // with a vertex obj only created from a tip info
            existing.parentIDsByType = vert.parentIDsByType;
            vert = existing;
        } else {
            if (vert.is_finalized) {
                this.finalized_count++;
            }
            this.verticesIncomingOrder.push(vert.id);
            this.checkLimit();
        }

        this.vertices.set(vert.id, vert);
        if (this.draw) {
            this.drawVertex(vert);
        }
    };

    @action
    addTipInfo = (tipInfo: TipInfo) => {
        let vert = this.vertices.get(tipInfo.id);
        if (!vert) {
            // create a new empty one for now
            vert = new Vertex();
            vert.id = tipInfo.id;
            this.verticesIncomingOrder.push(vert.id);
            this.checkLimit();
        }
        this.tips_count += tipInfo.is_tip ? 1 : vert.is_tip ? -1 : 0;
        vert.is_tip = tipInfo.is_tip;
        this.vertices.set(vert.id, vert);
        if (this.draw) {
            this.drawVertex(vert);
        }
    };

    @action
    checkLimit = () => {
        while (this.verticesIncomingOrder.length > this.verticesLimit) {
            let deleteId = this.verticesIncomingOrder.shift();
            let vert = this.vertices.get(deleteId);
            // make sure we remove any markings if the vertex gets deleted
            if (this.selected && deleteId === this.selected.id) {
                this.clearSelected();
            }
            this.vertices.delete(deleteId);
            if (this.draw) {
                this.graph.removeNode(deleteId);
            }
            if (!vert) {
                continue;
            }
            if (vert.is_finalized) {
                this.finalized_count--;
            }
            if (vert.is_tip) {
                this.tips_count--;
            }

            Object.keys(vert.parentIDsByType).map((parentType) => {
                vert.parentIDsByType[parentType].forEach((value) => {
                    this.deleteParentLink(value)
                })
            })
        }
    }

    @action
    deleteParentLink = (parentId: string) => {
        if (!parentId) {
            return;
        }
        let parent = this.vertices.get(parentId);
        if (parent) {
            if (this.selected && parentId === this.selected.id) {
                this.clearSelected();
            }
            if (parent.is_finalized) {
                this.finalized_count--;
            }
            if (parent.is_tip) {
                this.tips_count--;
            }
            this.vertices.delete(parentId);
        }
        if (this.draw) {
            this.graph.removeNode(parentId);
        }
    }

    drawVertex = (vert: Vertex) => {
        let node;
        let existing = this.graph.getNode(vert.id);
        if (existing) {
            // update coloring
            let nodeUI = this.graphics.getNodeUI(vert.id);
            nodeUI.color = parseColor(this.colorForVertexState(vert));
            node = existing
        } else {
            node = this.graph.addNode(vert.id, vert);
        }
        Object.keys(vert.parentIDsByType).map((parentType) => {
            vert.parentIDsByType[parentType].forEach((value) => {
                // if value is valid AND (links is empty OR there is no between parent and children)
                if (value && ((!node.links || !node.links.some(link => link.fromId === value)))) {
                    // draw the link only when the parent exists
                    let existing = this.graph.getNode(value);
                    if (existing) {
                        this.graph.addLink(value, vert.id);
                    }
                }
            })
        })
    }

    colorForVertexState = (vert: Vertex) => {
        if (!vert || !vert.parentIDsByType || Object.keys(vert.parentIDsByType).length === 0) return "#b58900";
        if (vert.is_tip) {
            return "#cb4b16";
        }
        // finalized
        if (vert.is_finalized) {
            if (vert.is_tx) {
                return "#fad02c";
            }
            return "#6c71c4"
        }

        // pending
        if (vert.is_tx) {
            return "#393e46"
        }
        return "#b9b7bd";
    }

    start = () => {
        this.draw = true;
        this.graph = Viva.Graph.graph();

        let graphics: any = Viva.Graph.View.webglGraphics();

        const layout = Viva.Graph.Layout.forceDirected(this.graph, {
            springLength: 10,
            springCoeff: 0.0001,
            stableThreshold: 0.15,
            gravity: -2,
            dragCoeff: 0.02,
            timeStep: 20,
            theta: 0.8,
        });

        graphics.node((node) => {
            if (!node.data) {
                return Viva.Graph.View.webglSquare(10, this.colorForVertexState(node.data));
            }
            return Viva.Graph.View.webglSquare(vertexSize, this.colorForVertexState(node.data));
        })
        graphics.link(() => Viva.Graph.View.webglLine("#586e75"));
        let ele = document.getElementById('visualizer');
        this.renderer = Viva.Graph.View.renderer(this.graph, {
            container: ele, graphics, layout,
        });

        let events = Viva.Graph.webglInputEvents(graphics, this.graph);

        events.mouseEnter((node) => {
            this.clearSelected(true);
            this.updateSelected(node.data);
        }).mouseLeave((node) => {
            this.clearSelected(false);
        });

        events.click((node) => {
            this.clearSelected(true);
            this.updateSelected(node.data, true);
        });

        this.graphics = graphics;
        this.renderer.run();

        this.vertices.forEach((vertex) => {
            this.drawVertex(vertex)
        })
    }

    stop = () => {
        this.draw = false;
        this.renderer.dispose();
        this.graph = null;
        this.paused = false;
        this.selected = null;
    }

    @action
    updateSelected = (vert: Vertex, viaClick?: boolean) => {
        if (!vert) return;

        this.selected = vert;
        this.selected_via_click = !!viaClick;

        // mutate links
        let nodeUI = this.graphics.getNodeUI(vert.id);
        this.selected_origin_color = nodeUI.color
        nodeUI.color = parseColor("#859900");
        nodeUI.size = vertexSize * 1.5;
    }

    resetLinks = () => {
        this.graph.forEachLink(function (link) {
            const linkUI = this.graphics.getLinkUI(link.id);
            linkUI.color = parseColor("#586e75");
        });
    }

    @action
    clearSelected = (force_clear?: boolean) => {
        if (!this.selected || (this.selected_via_click && !force_clear)) {
            return;
        }

        // clear link highlight
        let node = this.graph.getNode(this.selected.id);
        if (!node) {
            // clear links
            this.resetLinks();
            return;
        }

        let nodeUI = this.graphics.getNodeUI(this.selected.id);
        nodeUI.color = this.selected_origin_color;
        nodeUI.size = vertexSize;

        const seenForward = [];
        const seenBackwards = [];
        dfsIterator(this.graph, node, node => {
        }, true,
            link => {
                const linkUI = this.graphics.getLinkUI(link.id);
                linkUI.color = parseColor("#586e75");
            },
            seenBackwards
        );
        dfsIterator(this.graph, node, node => {
        }, false,
            link => {
                const linkUI = this.graphics.getLinkUI(link.id);
                linkUI.color = parseColor("#586e75");
            },
            seenForward
        );

        this.selected = null;
        this.selected_via_click = false;
    }

}

export default VisualizerStore;

// copied over and refactored from https://github.com/glumb/IOTAtangle
function dfsIterator(graph, node, cb, up, cbLinks: any = false, seenNodes = []) {
    seenNodes.push(node);
    let pointer = 0;

    while (seenNodes.length > pointer) {
        const node = seenNodes[pointer++];

        if (cb(node)) return true;

        for (const link of node.links) {
            if (cbLinks) cbLinks(link);

            if (!up && link.toId === node.id && !seenNodes.includes(graph.getNode(link.fromId))) {
                seenNodes.push(graph.getNode(link.fromId));
                continue;
            }

            if (up && link.fromId === node.id && !seenNodes.includes(graph.getNode(link.toId))) {
                seenNodes.push(graph.getNode(link.toId));
            }
        }
    }
}

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
