export var dagreOptions = {
    name: 'dagre',
    // dagre algo options, uses default value on undefined
    nodeSep: undefined, // the separation between adjacent nodes in the same rank
    edgeSep: undefined, // the separation between adjacent edges in the same rank
    rankSep: undefined, // the separation between each rank in the layout
    rankDir: 'LR', // 'TB' for top to bottom flow, 'LR' for left to right,
    ranker: undefined, // Type of algorithm to assign a rank to each node in the input graph. Possible values: 'network-simplex', 'tight-tree' or 'longest-path'
    minLen: function() {
        return 1;
    }, // number of ranks to keep between the source and target of the edge
    edgeWeight: function() {
        return 1;
    }, // higher weight edges are generally made shorter and straighter than lower weight edges

    // general layout options
    fit: true, // whether to fit to viewport
    padding: 30, // fit padding
    spacingFactor: undefined, // Applies a multiplicative factor (>0) to expand or compress the overall area that the nodes take up
    nodeDimensionsIncludeLabels: false, // whether labels should be included in determining the space used by a node
    animate: false, // whether to transition the node positions
    animationDuration: 500, // duration of animation in ms if enabled
    animationEasing: undefined, // easing of animation if enabled
    boundingBox: undefined, // constrain layout bounds; { x1, y1, x2, y2 } or { x1, y1, w, h }
    transform: function(node, pos) {
        return pos;
    }, // a function that applies a transform to the final node position
    ready: function() {
        return true;
    }, // on layoutready
    stop: function() {
        return true;
    } // on layoutstop
};

export var branchDagreOptions = {
    name: 'dagre',
    // dagre algo options, uses default value on undefined
    nodeSep: undefined, // the separation between adjacent nodes in the same rank
    edgeSep: undefined, // the separation between adjacent edges in the same rank
    rankSep: 200, // the separation between each rank in the layout
    rankDir: 'LR', // 'TB' for top to bottom flow, 'LR' for left to right,
    ranker: undefined, // Type of algorithm to assign a rank to each node in the input graph. Possible values: 'network-simplex', 'tight-tree' or 'longest-path'
    minLen: function() {
        return 1;
    }, // number of ranks to keep between the source and target of the edge
    edgeWeight: function() {
        return 1;
    }, // higher weight edges are generally made shorter and straighter than lower weight edges

    // general layout options
    fit: true, // whether to fit to viewport
    padding: 30, // fit padding
    spacingFactor: undefined, // Applies a multiplicative factor (>0) to expand or compress the overall area that the nodes take up
    nodeDimensionsIncludeLabels: false, // whether labels should be included in determining the space used by a node
    animate: false, // whether to transition the node positions
    animationDuration: 500, // duration of animation in ms if enabled
    animationEasing: undefined, // easing of animation if enabled
    boundingBox: undefined, // constrain layout bounds; { x1, y1, x2, y2 } or { x1, y1, w, h }
    transform: function(node, pos) {
        return pos;
    }, // a function that applies a transform to the final node position
    ready: function() {
        return true;
    }, // on layoutready
    stop: function() {
        return true;
    } // on layoutstop
};
