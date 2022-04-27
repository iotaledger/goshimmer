import { IGraph } from './graph';
import { default as Viva } from 'vivagraphjs';
import { parentRefType, tangleVertex } from 'models/tangle';
import {COLOR, CONFLICT, LINE_TYPE, LINE_WIDTH, VERTEX} from 'styles/tangleStyles';
import { ObservableMap } from 'mobx';

export class vivagraphLib implements IGraph {
    graph;
    graphics;
    layout;
    renderer;

    constructor(init: () => any) {
        [this.graph, this.layout, this.graphics, this.renderer] = init();
    }

    drawVertex(data: any): void {
        this.graph.addNode(data.ID, data);
    }

    removeVertex(id: string): void {
        this.graph.removeNode(id);
    }

    releaseNode(id: string): void {
        this.graphics.releaseNode(id);
    }

    selectVertex(id: string): void {
        // only size, border and position are changed.
        const node = this.graph.getNode(id);
        const nodeUI = this.graphics.getNodeUI(id);
        if (!nodeUI) {
            return;
        }
        setUINodeSize(nodeUI, VERTEX.SIZE_SELECTED);
        setRectBorder(
            nodeUI,
            VERTEX.SELECTED_BORDER_WIDTH,
            COLOR.NODE_BORDER_SELECTED
        );

        const pos = this.layout.getNodePosition(node.id);
        svgUpdateNodePos(nodeUI, pos);
    }

    unselectVertex(id: string): void {
        const nodeUI = this.graphics.getNodeUI(id);
        if (!nodeUI) {
            return;
        }
        setUINodeSize(nodeUI, VERTEX.SIZE_DEFAULT);
        resetRectBorder(nodeUI);
    }

    clearGraph(): void {
        this.graph.clear();
    }

    centerGraph(): void {
        const rect = this.layout.getGraphRect();
        const centerY = (rect.y1 + rect.y2) / 2;
        const centerX = (rect.x1 + rect.x2) / 2;

        this.renderer.moveTo(centerX, centerY);
    }

    centerVertex(id: string): void {
        const pos = this.layout.getNodePosition(id);
        this.renderer.moveTo(pos.x, pos.y);
    }

    stop(): void {
        this.renderer.dispose();
        this.graph = null;
    }

    pause(): void {
        this.renderer.pause();
    }

    resume(): void {
        this.renderer.resume();
    }

    getNodeColor(id: string): string {
        const nodeUI = this.graphics.getNodeUI(id);
        if (!nodeUI) {
            return '';
        }
        return getUINodeColor(nodeUI);
    }

    nodeExist(id: string): boolean {
        const nodeUI = this.graphics.getNodeUI(id);
        if (!nodeUI) {
            return false;
        }
        return true;
    }
}

export function initTangleDAG() {
    const graph = Viva.Graph.graph();
    const layout = setupLayout(graph);
    const graphics = setupSvgGraphics();
    const renderer = setupRenderer(graph, graphics, layout);
    renderer.run();
    maximizeSvgWindow();

    return [graph, layout, graphics, renderer];
}

function setupLayout(graph: any) {
    return Viva.Graph.Layout.forceDirected(graph, {
        springLength: 10,
        springCoeff: 0.0001,
        stableThreshold: 0.15,
        gravity: -2,
        dragCoeff: 0.02,
        timeStep: 20,
        theta: 0.8
    });
}

function setupSvgGraphics() {
    const graphics: any = Viva.Graph.View.svgGraphics();

    graphics
        .node(() => {
            return svgNodeBuilder();
        })
        .placeNode(svgUpdateNodePos);

    graphics
        .link(() => {
            return svgLinkBuilder(
                COLOR.LINK_STRONG,
                LINE_WIDTH.STRONG,
                LINE_TYPE.STRONG
            );
        })
        .placeLink(function(linkUI, fromPos, toPos) {
            // linkUI - is the object returned from link() callback above.
            const data =
                'M' +
                fromPos.x.toFixed(2) +
                ',' +
                fromPos.y.toFixed(2) +
                'L' +
                toPos.x.toFixed(2) +
                ',' +
                toPos.y.toFixed(2);

            // 'Path data' (http://www.w3.org/TR/SVG/paths.html#DAttribute )
            // is a common way of rendering paths in SVG:
            linkUI.attr('d', data);
        });

    return graphics;
}

function setupRenderer(graph: any, graphics: any, layout: any) {
    const ele = document.getElementById('tangleVisualizer');

    return Viva.Graph.View.renderer(graph, {
        container: ele,
        graphics: graphics,
        layout: layout
    });
}

export function drawMessage(
    msg: tangleVertex,
    vivaLib: vivagraphLib,
    messageMap: ObservableMap<string, tangleVertex>
) {
    let node;
    const existing = vivaLib.graph.getNode(msg.ID);
    if (existing) {
        node = existing;
    } else {
        node = vivaLib.graph.addNode(msg.ID, msg);
        updateNodeColorOnConfirmation(msg, vivaLib);
    }

    const drawVertexParentReference = (
        parentType: parentRefType,
        parentIDs: Array<string>
    ) => {
        if (parentIDs) {
            parentIDs.forEach((value) => {
                // remove tip status
                const parent = messageMap.get(value);
                if (parent) {
                    parent.isTip = false;
                    updateNodeColorOnConfirmation(parent, vivaLib);
                }

                // if value is valid AND (links is empty OR there is no between parent and children)
                if (
                    value &&
                    (!node.links ||
                        !node.links.some((link) => link.fromId === value))
                ) {
                    // draw the link only when the parent exists
                    const existing = vivaLib.graph.getNode(value);
                    if (existing) {
                        const link = vivaLib.graph.addLink(value, msg.ID);
                        updateParentRefUI(link.id, vivaLib, parentType);
                    }
                }
            });
        }
    };
    drawVertexParentReference(parentRefType.StrongRef, msg.strongParentIDs);
    drawVertexParentReference(parentRefType.WeakRef, msg.weakParentIDs);
    drawVertexParentReference(parentRefType.ShallowLikeRef, msg.shallowLikeParentIDs);
    drawVertexParentReference(parentRefType.ShallowDislikeRef, msg.shallowDislikeParentIDs);
}

export function selectMessage(id: string, vivaLib: vivagraphLib) {
    vivaLib.selectVertex(id);

    const node = vivaLib.graph.getNode(id);
    const nodeUI = vivaLib.graphics.getNodeUI(id);
    if (!nodeUI) {
        return;
    }

    setUINodeColor(nodeUI, COLOR.NODE_SELECTED);

    const seenForward = [];
    const seenBackwards = [];
    dfsIterator(
        vivaLib.graph,
        node,
        () => {
            return false;
        },
        true,
        (link) => {
            const linkUI = vivaLib.graphics.getLinkUI(link.id);
            setUILinkColor(linkUI, COLOR.LINK_FUTURE_CONE);
        },
        seenForward
    );
    dfsIterator(
        vivaLib.graph,
        node,
        () => {
            return false;
        },
        false,
        (link) => {
            const linkUI = vivaLib.graphics.getLinkUI(link.id);
            setUILinkColor(linkUI, COLOR.LINK_PAST_CONE);
        },
        seenBackwards
    );

    return node;
}

export function unselectMessage(
    id: string,
    originColor: string,
    vivaLib: vivagraphLib
) {
    vivaLib.unselectVertex(id);

    // clear link highlight
    const node = vivaLib.graph.getNode(id);
    if (!node) {
        // clear links
        resetLinks(vivaLib);
        return;
    }

    const nodeUI = vivaLib.graphics.getNodeUI(id);
    setUINodeColor(nodeUI, originColor);

    const seenForward = [];
    const seenBackwards = [];
    dfsIterator(
        vivaLib.graph,
        node,
        () => {
            return false;
        },
        true,
        (link) => {
            updateParentRefUI(link.id, vivaLib);
        },
        seenBackwards
    );
    dfsIterator(
        vivaLib.graph,
        node,
        () => {
            return false;
        },
        false,
        (link) => {
            updateParentRefUI(link.id, vivaLib);
        },
        seenForward
    );
}

export function updateGraph(
    vivaLib: vivagraphLib,
    newMsgToAdd: string[],
    messageMap: ObservableMap<string, tangleVertex>
) {
    vivaLib.graph.forEachNode((node) => {
        const msg = messageMap.get(node.id);
        if (!msg) {
            vivaLib.removeVertex(node.id);
        } else {
            updateNodeDataAndColor(msg.ID, msg, vivaLib);
        }
    });

    // new messages to add after pause.
    for (const msgID of newMsgToAdd) {
        const msg = messageMap.get(msgID);
        if (msg) {
            drawMessage(msg, vivaLib, messageMap);
            updateNodeColorOnConfirmation(msg, vivaLib);
        }
    }
}

// pause was short - clear only the needed part on left from this.lastMsgAddedBeforePause
export function reloadAfterShortPause(
    vivaLib: vivagraphLib,
    messageMap: ObservableMap<string, tangleVertex>
) {
    vivaLib.graph.forEachNode((node) => {
        const msg = messageMap.get(node.id);
        if (!msg) {
            vivaLib.graph.removeNode(node.id);
        } else {
            updateNodeDataAndColor(msg.ID, msg, vivaLib);
        }
    });
}

export function updateNodeDataAndColor(
    nodeID: string,
    msgData: tangleVertex,
    vivaLib: vivagraphLib
) {
    const exists = vivaLib.nodeExist(nodeID);
    // replace existing node data
    if (exists && msgData) {
        vivaLib.drawVertex(msgData);
        updateNodeColorOnConfirmation(msgData, vivaLib);
        if (msgData.isMarker) {
            drawMarker(msgData.ID, vivaLib);
        }

    }

}

function drawMarker(id: string, vivaLib: vivagraphLib) {
    const group = vivaLib.graphics.getNodeUI(id);
    // check if the node already has a marker.
    if (group.markerAdded) {
        return;
    }

    const circle = Viva.Graph.svg('circle');
    circle
        .attr('fill', COLOR.MARKER)
        .attr('r', VERTEX.MARKER_SIZE)
        .attr('cx', VERTEX.SIZE_DEFAULT / 2)
        .attr('cy', VERTEX.SIZE_DEFAULT / 2);

    group.markerAdded = true;
    group.append(circle);
}

function drawRejectMark(id: string, vivaLib: vivagraphLib) {
    const group = vivaLib.graphics.getNodeUI(id);

    const x = VERTEX.SIZE_DEFAULT/5;
    const y = VERTEX.SIZE_DEFAULT/4;
    const dx = VERTEX.SIZE_DEFAULT/8;
    const dy = VERTEX.SIZE_DEFAULT/8;
    const mark = Viva.Graph.svg('polyline');
    mark
        .attr('stroke', CONFLICT.LOOSER_COLOR)
        .attr('stroke-width',  CONFLICT.WIDTH)
        .attr('points', `${-x+dx}, ${y+dy} ${dx}, ${dy} ${x+dx}, ${y+dy} ${-x+dx}, ${-y+dy}  ${dx},
          ${dy} ${x+dx}, ${-y+dy}`);

    group.append(mark);
}

function drawWinnerMark(id: string, vivaLib: vivagraphLib) {
    const group = vivaLib.graphics.getNodeUI(id);

    const x = VERTEX.SIZE_DEFAULT/5;
    const y = VERTEX.SIZE_DEFAULT/4;
    const dx = VERTEX.SIZE_DEFAULT/8;
    const dy = VERTEX.SIZE_DEFAULT/8;
    const mark = Viva.Graph.svg('polyline');
    mark
        .attr('stroke', CONFLICT.WINNER_COLOR)
        .attr('stroke-width',  CONFLICT.WIDTH)
        .attr('points', `${-x+dx}, ${-y+dy} ${dx}, ${dy} ${2*x+dx}, ${-2*y+dy} ${dx}, ${dy}`);

    group.append(mark);
}

function svgUpdateNodePos(nodeUI, pos) {
    const rectUI = nodeUI.childNodes[0];
    const size = rectUI.getAttribute('width');

    nodeUI.attr(
        'transform',
        'translate(' + (pos.x - size / 2) + ',' + (pos.y - size / 2) + ')'
    );
}

function resetLinks(vivaLib: vivagraphLib) {
    vivaLib.graph.forEachLink((link) => {
        updateParentRefUI(link.id, vivaLib);
    });
}

function updateNodeColorOnConfirmation(
    msg: tangleVertex,
    vivaLib: vivagraphLib
) {
    const nodeUI = vivaLib.graphics.getNodeUI(msg.ID);
    if (!nodeUI) return;
    if (msg.isTip) return;

    let color = '';
    color = msg.isTx ? COLOR.TRANSACTION_PENDING : COLOR.MESSAGE_PENDING;
    if (msg.isConfirmed) {
        color = msg.isTx
            ? COLOR.TRANSACTION_CONFIRMED
            : COLOR.MESSAGE_CONFIRMED;
    }
    if (msg.isTx && msg.isConfirmed) {
        msg.isTxConfirmed ? drawWinnerMark(msg.ID, vivaLib) : drawRejectMark(msg.ID, vivaLib);
    }
    setUINodeColor(nodeUI.childNodes[0], color);
}

function updateParentRefUI(
    linkID: string,
    vivaLib: vivagraphLib,
    parentType?: parentRefType
) {
    // update link line type and color based on reference type
    const linkUI = vivaLib.graphics.getLinkUI(linkID);
    if (!linkUI) {
        return;
    }
    // if type not provided look for refType data if not found use strong ref style
    if (parentType === undefined) {
        parentType = linkUI.refType || parentRefType.StrongRef;
    }

    switch (parentType) {
    case parentRefType.StrongRef: {
        setUILink(linkUI, COLOR.LINK_STRONG, LINE_WIDTH.STRONG, LINE_TYPE.STRONG);
        linkUI.refType = parentRefType.StrongRef;
        break;
    }
    case parentRefType.WeakRef: {
        setUILink(linkUI, COLOR.LINK_WEAK, LINE_WIDTH.WEAK, LINE_TYPE.WEAK);
        linkUI.refType = parentRefType.WeakRef;
        break;
    }
    case parentRefType.ShallowLikeRef: {
        setUILink(linkUI, COLOR.LINK_SHALLOW_LIKED, LINE_WIDTH.SHALLOW_LIKED, LINE_TYPE.SHALLOW_LIKED);
        linkUI.refType = parentRefType.ShallowLikeRef;
        break;
    }
    case parentRefType.ShallowDislikeRef: {
        setUILink(linkUI, COLOR.LINK_SHALLOW_DISLIKED, LINE_WIDTH.SHALLOW_DISLIKED, LINE_TYPE.SHALLOW_DISLIKED);
        linkUI.refType = parentRefType.ShallowDislikeRef;
        break;
    }
    }
}

// copied over and refactored from https://github.com/glumb/IOTAtangle
function dfsIterator(
    graph,
    node,
    cb,
    up,
    cbLinks: any = false,
    seenNodes = []
) {
    seenNodes.push(node);
    let pointer = 0;

    while (seenNodes.length > pointer) {
        const node = seenNodes[pointer++];

        if (cb(node)) return true;

        for (const link of node.links || []) {
            if (cbLinks) cbLinks(link);

            if (
                !up &&
                link.toId === node.id &&
                !seenNodes.includes(graph.getNode(link.fromId))
            ) {
                seenNodes.push(graph.getNode(link.fromId));
                continue;
            }

            if (
                up &&
                link.fromId === node.id &&
                !seenNodes.includes(graph.getNode(link.toId))
            ) {
                seenNodes.push(graph.getNode(link.toId));
            }
        }
    }
}

const svgNodeBuilder = function(): any {
    const group = Viva.Graph.svg('g');

    const ui = Viva.Graph.svg('rect');
    setUINodeColor(ui, COLOR.TIP);
    setUINodeSize(ui, VERTEX.SIZE_DEFAULT);
    setCorners(ui, VERTEX.ROUNDED_CORNER);

    group.append(ui);

    return group;
};

const svgLinkBuilder = function(color: string, width: number, type: string) {
    return Viva.Graph.svg('path')
        .attr('stroke', color)
        .attr('stroke-width', width)
        .attr('stroke-dasharray', type);
};

function maximizeSvgWindow() {
    const svgEl = document.querySelector('#tangleVisualizer>svg');
    svgEl.setAttribute('width', '100%');
    svgEl.setAttribute('height', '100%');
}

function setUINodeColor(ui: any, color: any) {
    ui.attr('fill', color);
}

function setUILinkColor(ui: any, color: any) {
    ui.attr('stroke', color);
}

function getUINodeColor(ui: any): string {
    return ui.getAttribute('fill');
}

function setUINodeSize(ui: any, size: number) {
    ui.attr('width', size);
    ui.attr('height', size);
}

function setUILink(ui: any, color: string, width: number, type: string) {
    ui.attr('stroke-width', width);
    ui.attr('stroke-dasharray', type);
    ui.attr('stroke', color);
}

function setCorners(ui: any, rx: number) {
    ui.attr('rx', rx);
}

function setRectBorder(ui: any, borderWidth: number, borderColor) {
    ui.attr('stroke-width', borderWidth);
    ui.attr('stroke', borderColor);
}

function resetRectBorder(ui: any) {
    ui.removeAttribute('stroke-width');
    ui.removeAttribute('stroke');
}
