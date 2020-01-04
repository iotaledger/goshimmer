document.onkeydown = function (e) {
    var keyCode = e.keyCode;
    if(keyCode == 13) {
        startButton.checked = true;
        animation()
    }
};
 
 function animation() {
    setTimeout(onStart, 500);
 }

 function onStart() {
    wrapper = document.getElementById("wrapperID");
    wrapper.style.display="none";
    onLoad();
    sendStart();
}

function sendStart() {
    fetch("http://localhost:8844/start")
    .then(res=>{console.log(res)})
}

function onLoad() {
    var nodepool = [];
    var nodesonline = [];
    var nodesoffline = [];
    var connections = [];
    var nodesdisconnected = [];
    var nodesonlinesel = [];
    var previousnodes = [];
    var previouslinks = [];

    var cfactive = 0;
    var rendered = 0;

    var floodold = 0;
    var floodnew = 0;

    var graph = Viva.Graph.graph();
    var graphics = Viva.Graph.View.webglGraphics();

    var layout = Viva.Graph.Layout.forceDirected(graph, {
        springLength: 30,
        springCoeff: 0.0001,
        dragCoeff: 0.02,
        gravity: -1.2
    });

    graphics.link(function (link) {
            return Viva.Graph.View.webglLine("#444444");
        });

    var circleNode = buildCircleNodeShader();
    graphics.setNodeProgram(circleNode);

    var nodeColor = 0x666666,			
        nodeSize = 14;

    graphics.node(function (node) {
        return new WebglCircle(nodeSize, nodeColor);
    });

    var renderer = Viva.Graph.View.renderer(graph,
        {
            layout: layout,
            graphics: graphics,
            container: document.getElementById('graphc'),
            renderLinks: true
        });

    var events = Viva.Graph.webglInputEvents(graphics, graph);

    events.mouseEnter(function (node) {

        graph.beginUpdate();

        if (cfactive == 1) {

            resetColors();
            document.getElementById("ctxt").innerHTML = "";
            document.getElementById("scale").style.display = "none";
            cfactive = 0;

        }

        if (previousnodes.length > 0) {
            resetPreviousColors();
        }

        showNodeLinks(node.id);

        graph.endUpdate();
        renderer.rerender();

    }).mouseLeave(function (node) {

    }).dblClick(function (node) {

    }).click(function (node) {

    });

    var a = document.getElementById('onlinenodes');
    if (a != null) { 
        a.addEventListener('click', function () {
            if(rendered == 1){
                nodesonlinesel = nodesonline;
                nodesonlinesel.sort();

                var sel = document.getElementById('nds');
                sel.innerHTML = "";

                for (var i = 0; i < nodesonlinesel.length; i++) {
                    var opt = document.createElement('option');
                    opt.innerHTML = nodesonlinesel[i];
                    opt.value = nodesonlinesel[i];
                    sel.appendChild(opt);
                }
            }
        }, false);
    }

    var b = document.getElementById('onlinenodes');
    if (b != null) { 
        b.addEventListener('change', function () {

            var selected = event.target.value;
            document.getElementById("onlinenodes").value = "info";

            graph.beginUpdate();

            if (cfactive == 1) {

                resetColors();
                document.getElementById("ctxt").innerHTML = "";
                document.getElementById("scale").style.display = "none";
                cfactive = 0;            

            }

            if (previousnodes.length > 0) {
                resetPreviousColors();
            }

            if (selected == "cc" || selected == "bc" || selected == "dc" || selected == "ec" || selected == "cw") {
                cfactive = 1;
                if (selected == "cw") {
                    chw(graph, graphics);
                } else {
                    centralityFunc(selected);
                }

            } else {

                showNodeLinks(selected);

            }

            graph.endUpdate();
            renderer.rerender();

        }, false);
    }

    var socket = new WebSocket(((window.location.protocol === "https:") ? "wss://" : "ws://") + "localhost:8844" + "/ws");
    var myVar = setInterval(myTimer, 500);

    socket.onopen = function () {
        document.getElementById("stat").innerHTML = "WebSocket opened. Loading ... ";
        setInterval(function () {
            socket.send("_");
        }, 1000);
    };

    socket.onerror = function(event) {
        document.getElementById("stat").innerHTML = "WebSocket error observed", event;
        console.log("WebSocket error observed", event);
      };

    socket.onmessage = function (e) {
        if(rendered == 0){floodnew++;}
        var eventArr = e.data.split(" ");
        //console.log(eventArr);

        switch (eventArr[0]) {
            case "1":
                addNode(eventArr[1]);
                setNodeOnline(eventArr[1]);  
                break;

            case "2":               
                //removeNode(eventArr[1]);
                break;

            case "3":
                connectNodes(eventArr[1], eventArr[2])
                break;

            case "4":
                disconnectNodes(eventArr[1], eventArr[2])
                disconnectNodes(eventArr[2], eventArr[1])
                break;

            case "5":
                document.getElementById("convergence").innerHTML = "Convergence: " + eventArr[1] + "%";
                break;
                
            case "6":
                document.getElementById("avgNeighbors").innerHTML = "Avg # of neighbors: " + eventArr[1];
                break;
        }
    }

    function myTimer() {
        if (floodnew > floodold + 100) {
            document.getElementById("streamstat").innerHTML = "... received " + floodnew + " msg";
            floodold = floodnew;

        } else {
            myStopFunction();
            rendered++;
            renderer.run();
            calculateDiscNodes();
            document.getElementById("streamstat").innerHTML = "... received " + floodnew + " msg";
            document.getElementById("stat").innerHTML = "nodes online: " + nodesonline.length + "(" + nodesdisconnected.length + ")" + " - IDs: " + nodepool.length + " - edges: " + connections.length;
            //document.getElementById("onlinenodesp").style.display = "block";


            if (window.location.hash) {
                var hash = window.location.hash.substring(1);
                showNodeLinks(hash);
            } 

        }

    }

    function myStopFunction() {
        clearInterval(myVar);
    }
    
    function addNode(idA) {
        var index = nodepool.indexOf(idA);
        if (index == -1) {
            nodepool.push(idA);
            if (rendered == 1) {
                document.getElementById("streamstat").innerHTML = "addedToNodepool: " + idA;
                document.getElementById("stat").innerHTML = "nodes online: " + nodesonline.length + "(" + nodesdisconnected.length + ")" + " - IDs: " + nodepool.length + " - edges: " + connections.length;
            }
        }
    }

    async function connectNodes(idA, idB) {
        var indexc = connections.indexOf(idA + idB);
        if (indexc > -1) {
            if (rendered == 1) { document.getElementById("streamstat").innerHTML = "connectNodes skipped: " + idA + " > " + idB; }
            //document.getElementById("stat").style.color = "black";			
        } else {
            var index = nodesonline.indexOf(idA);
            var index2 = nodesonline.indexOf(idB);

            if (index > -1 && index2 > -1 && idA != idB) {

                var x = idA + idB;
                link = graph.addLink(idA, idB, x);
                connections.push(idA + idB);
                if (rendered == 1) {
                    calculateDiscNodes();
                    //console.log("connectNodes: " + idA + " > " + idB);
                    document.getElementById("streamstat").innerHTML = "connectNodes: " + idA + " > " + idB;
                    document.getElementById("stat").innerHTML = "nodes online: " + nodesonline.length + "(" + nodesdisconnected.length + ")" + " - IDs: " + nodepool.length + " - edges: " + connections.length;
                }

                var nodeUI = graphics.getNodeUI(idA);
                if (nodeUI != null) {
                    nodeUI.color = "0x336DB5";  
                }
                nodeUI = graphics.getNodeUI(idB);
                if (nodeUI != null) {
                    nodeUI.color = "0x1C8D7F";
                }

                var linkUI = graphics.getLinkUI(link.id);
                var colorl = parseColor('#336DB5');
                linkUI.color = colorl;
                
                await sleep(100);
                colorl = parseColor('#444444');
                linkUI.color = colorl;
                nodeUI = graphics.getNodeUI(idA);
                //if (nodeUI != null) {
                    nodeUI.color = "0x666666";  
                //}
                nodeUI = graphics.getNodeUI(idB);
                if (nodeUI != null) {
                    nodeUI.color = "0x666666";
                }
            }

            //document.getElementById("stat").style.color = "green"; 

        }

    }

    async function disconnectNodes(idA, idB) {
        var indexc = connections.indexOf(idA + idB);
        if (indexc != -1) {
            var conn = graph.getLink(idA, idB);
            connections.splice(indexc, 1);

            if (rendered == 1) {
                calculateDiscNodes();
                document.getElementById("streamstat").innerHTML = "disconnectNodes: " + idA + " > " + idB;
                document.getElementById("stat").innerHTML = "nodes online: " + nodesonline.length + "(" + nodesdisconnected.length + ")" + " - IDs: " + nodepool.length + " - edges: " + connections.length;
            }

            var nodeUI = graphics.getNodeUI(idA);
            if (nodeUI != null) {
                nodeUI.color = "0xFF0000";  
            }
            nodeUI = graphics.getNodeUI(idB);
            if (nodeUI != null) {
                nodeUI.color = "0xFF0000";
            }

            var linkUI = graphics.getLinkUI(conn.id);
            var colorl = parseColor('#FF0000');
            linkUI.color = colorl;
            
            await sleep(100);
            nodeUI = graphics.getNodeUI(idA);
            if (nodeUI != null) {
                nodeUI.color = "0x666666";  
            }
            nodeUI = graphics.getNodeUI(idB);
            if (nodeUI != null) {
                nodeUI.color = "0x666666";
            }

            graph.removeLink(conn);
        }
    }

    function setNodeOnline(idA) {

        var index = nodesonline.indexOf(idA);
        if (index == -1) {

            nodesonline.push(idA);
            graph.addNode(idA);

            if (rendered == 1) {

                calculateDiscNodes();
                //console.log("setNodeOnline: " + idA);
                document.getElementById("streamstat").innerHTML = "setNodeOnline: " + idA;
                document.getElementById("stat").innerHTML = "nodes online: " + nodesonline.length + "(" + nodesdisconnected.length + ")" + " - IDs: " + nodepool.length + " - edges: " + connections.length;
            }
        } else {
            //console.log("setNodeOnline skipped: " + idA);
            if (rendered == 1) { document.getElementById("streamstat").innerHTML = "setNodeOnline skipped: " + idA; }
        }

        var index2 = nodesoffline.indexOf(idA)
        if (index2 > -1) {
            nodesoffline.splice(index2, 1);
            //console.log("removedFromOfflinepool: " + idA);
            if (rendered == 1) {
                document.getElementById("streamstat").innerHTML = "removedFromOfflinepool: " + idA;
                document.getElementById("stat").innerHTML = "nodes online: " + nodesonline.length + "(" + nodesdisconnected.length + ")" + " - IDs: " + nodepool.length + " - edges: " + connections.length;
            }
        }

        //document.getElementById("stat").style.color = "green";

    }

    function calculateDiscNodes() {

        nodesdisconnected.splice(0, nodesdisconnected.length);
        graph.forEachNode(function (node) {
            var lc = 0;
            graph.forEachLinkedNode(node.id, function (linkedNode, link) {
                lc++;
            });
            if (lc == 0) { nodesdisconnected.push(node.id) }
        });

    }

    function resetColors() {

        graph.forEachNode(function (node) {

            var nodeUI = graphics.getNodeUI(node.id);
            if (nodeUI != null) {
                nodeUI.color = "0x666666";
            }

        });

        graph.forEachLink(function (link) {
            var linkUI = graphics.getLinkUI(link.id);
            var colorl = parseColor('#444444');
            if (linkUI != null) {
                linkUI.color = colorl;

            }
        });

    }

    function resetPreviousColors() {

        for (var i = 0; i < previousnodes.length; i++) {
            var nodeid = previousnodes[i];

            var nodeUI = graphics.getNodeUI(nodeid);
            if (nodeUI != null) {
                nodeUI.color = "0x666666";
            } else {

            }

        }

        previousnodes.splice(0, previousnodes.length);

        for (var j = 0; j < previouslinks.length; j++) {
            var linkid = previouslinks[j];
            var linkUI = graphics.getLinkUI(linkid);
            if (linkUI != null) {
                var colorl = parseColor("#444444");
                linkUI.color = colorl;
            } else {

            }
        }

        previouslinks.splice(0, previouslinks.length);

    }

    function showNodeLinks(selected) {

        document.getElementById("nodestath").innerHTML = "";
        document.getElementById("in").innerHTML = "incoming edges: ";
        document.getElementById("out").innerHTML = "outgoing edges: ";

        var nb = 0;
        var inc = 0;
        var outc = 0;

        var nodeUI = graphics.getNodeUI(selected);
        if (nodeUI != null) {
            nodeUI.color = "0x336db5";
        }

        previousnodes.push(selected);
        graph.forEachLinkedNode(selected, function (linkedNode, link) {
            nb++;

            var nodeid = selected;
            var links = link.data;

            if (links.startsWith(nodeid) === true) {
                document.getElementById("out").innerHTML += /*link.data.substr(0, 16)*/ "NODE" + " &rarr; " + link.data.substr(16, 32) + "<br>";
                outc++;

                var linkUI = graphics.getLinkUI(link.id);
                var colorl = parseColor("#336db5");
                if (linkUI != null) {
                    linkUI.color = colorl;
                    previouslinks.push(link.id);
                }

            } else {
                document.getElementById("in").innerHTML += link.data.substr(0, 16) + " &rarr; " + /*link.data.substr(16, 32)*/ "NODE" + "<br>";
                inc++;

                var linkUI = graphics.getLinkUI(link.id);
                var colorl = parseColor("#1c8d7f");
                if (linkUI != null) {
                    linkUI.color = colorl;
                    previouslinks.push(link.id);
                }
            }

            var nodeUIL = graphics.getNodeUI(linkedNode.id);
            if (nodeUIL != null) {
                nodeUIL.color = "0x1c8d7f";
                previousnodes.push(linkedNode.id);
            }

        });

        if(inc == 0){document.getElementById("in").innerHTML += "no incoming edges!"};
        if(outc == 0){document.getElementById("out").innerHTML += "no outgoing edges!"};

        document.getElementById("nodestath").innerHTML = "NODE ID: " + selected + " in:" + inc + " out:" + outc;       

    }

}
/*### color parser ###*/

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

/*### webgl circle ###*/

// Lets start from the easiest part - model object for node ui in webgl
function WebglCircle(size, color) {
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

function sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}

