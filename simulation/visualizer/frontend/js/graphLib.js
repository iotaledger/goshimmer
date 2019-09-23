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
    var calculator = Viva.Graph.centrality();

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
                removeNode(eventArr[1]);
                break;

            case "3":
                connectNodes(eventArr[1], eventArr[2]);
                break;

            case "4":
                disconnectNodes(eventArr[1], eventArr[2]);
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

    function removeNode(idA) {
        var index = nodesonline.indexOf(idA);
        if (index > -1) {
            nodesonline.splice(index, 1);
            graph.removeNode(idA);
            //console.log("removeNode from onlinepool: " + idA);
            if (rendered == 1) {
                document.getElementById("streamstat").innerHTML = "removeNode from onlinepool: " + idA;
                document.getElementById("stat").innerHTML = "nodes online: " + nodesonline.length + "(" + nodesdisconnected.length + ")" + " - IDs: " + nodepool.length + " - edges: " + connections.length;
            }
        }

        var index2 = nodesoffline.indexOf(idA);
        if (index2 > -1) {
            nodesoffline.splice(index2, 1);
            //console.log("removeNode from offlinepool: " + idA);
            if (rendered == 1) {
                document.getElementById("streamstat").innerHTML = "removeNode from offlinepool: " + idA;
                document.getElementById("stat").innerHTML = "nodes online: " + nodesonline.length + "(" + nodesdisconnected.length + ")" + " - IDs: " + nodepool.length + " - edges: " + connections.length;
            }
        }

        var index3 = nodepool.indexOf(idA);
        if (index3 > -1) {
            nodepool.splice(index3, 1);
            if (rendered == 1) {
                //console.log("removeNode from offlinepool: " + idA);
                document.getElementById("streamstat").innerHTML = "removeNode from nodepool: " + idA;
                document.getElementById("stat").innerHTML = "nodes online: " + nodesonline.length + "(" + nodesdisconnected.length + ")" + " - IDs: " + nodepool.length + " - edges: " + connections.length;
            }
        }
        //document.getElementById("stat").style.color = "red"; 
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


                connections.push(idA + idB);

                if (rendered == 1) {
                    calculateDiscNodes();
                    //console.log("connectNodes: " + idA + " > " + idB);
                    document.getElementById("streamstat").innerHTML = "connectNodes: " + idA + " > " + idB;
                    document.getElementById("stat").innerHTML = "nodes online: " + nodesonline.length + "(" + nodesdisconnected.length + ")" + " - IDs: " + nodepool.length + " - edges: " + connections.length;
                }
            }

            //document.getElementById("stat").style.color = "green"; 

        }

    }

    async function disconnectNodes(idA, idB) {

        var indexc = connections.indexOf(idA + idB);
        if (indexc > -1) {
            var conn = graph.getLink(idA, idB);

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
            connections.splice(indexc, 1);

            if (rendered == 1) {
                calculateDiscNodes();
                document.getElementById("streamstat").innerHTML = "disconnectNodes: " + idA + " > " + idB;
                document.getElementById("stat").innerHTML = "nodes online: " + nodesonline.length + "(" + nodesdisconnected.length + ")" + " - IDs: " + nodepool.length + " - edges: " + connections.length;
            }
        }

        //document.getElementById("stat").style.color = "red";

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

    function centralityFunc(selected){

        document.getElementById("in").innerHTML = "";
        document.getElementById("out").innerHTML = "";
        document.getElementById("nodestath").innerHTML = "";
        document.getElementById("scale").style.display = "block";

        var tmpArr = [];
        var t0 = performance.now();

        if (selected == "cc") {
            var closeness = calculator.closenessCentrality(graph, true);
            tmpArr = closeness;
            document.getElementById("ctxt").innerHTML = "snapshot: closeness centrality";
        };

        if (selected == "bc") {
            var betweenness = calculator.betweennessCentrality(graph, true);
            tmpArr = betweenness;
            document.getElementById("ctxt").innerHTML = "snapshot: betweenness centrality";
        };

        if (selected == "dc") {
            var degree = calculator.degreeCentrality(graph, 'inout');
            tmpArr = degree;
            document.getElementById("ctxt").innerHTML = "snapshot: degree centrality";
        };

        if (selected == "ec") {
            var eccentricity = calculator.eccentricityCentrality(graph, true);
            tmpArr = eccentricity;
            document.getElementById("ctxt").innerHTML = "snapshot: eccentricity centrality";
        };


        var toNormalize = [];

        for (var i = 0; i < tmpArr.length; i++) {
            toNormalize.push(tmpArr[i].value);
        }

        var ratiob = Math.max(...toNormalize) / 255;
        toNormalize = toNormalize.map(v => (v / ratiob).toFixed(0));

        var colorM = ["0x000004", "0x010005", "0x010106", "0x010108", "0x020109", "0x02020b", "0x02020d", "0x03030f", "0x030312", "0x040414", "0x050416", "0x060518", "0x06051a", "0x07061c", "0x08071e", "0x090720", "0x0a0822", "0x0b0924", "0x0c0926", "0x0d0a29", "0x0e0b2b", "0x100b2d", "0x110c2f", "0x120d31", "0x130d34", "0x140e36", "0x150e38", "0x160f3b", "0x180f3d", "0x19103f", "0x1a1042", "0x1c1044", "0x1d1147", "0x1e1149", "0x20114b", "0x21114e", "0x221150", "0x241253", "0x251255", "0x271258", "0x29115a", "0x2a115c", "0x2c115f", "0x2d1161", "0x2f1163", "0x311165", "0x331067", "0x341069", "0x36106b", "0x38106c", "0x390f6e", "0x3b0f70", "0x3d0f71", "0x3f0f72", "0x400f74", "0x420f75", "0x440f76", "0x451077", "0x471078", "0x491078", "0x4a1079", "0x4c117a", "0x4e117b", "0x4f127b", "0x51127c", "0x52137c", "0x54137d", "0x56147d", "0x57157e", "0x59157e", "0x5a167e", "0x5c167f", "0x5d177f", "0x5f187f", "0x601880", "0x621980", "0x641a80", "0x651a80", "0x671b80", "0x681c81", "0x6a1c81", "0x6b1d81", "0x6d1d81", "0x6e1e81", "0x701f81", "0x721f81", "0x732081", "0x752181", "0x762181", "0x782281", "0x792282", "0x7b2382", "0x7c2382", "0x7e2482", "0x802582", "0x812581", "0x832681", "0x842681", "0x862781", "0x882781", "0x892881", "0x8b2981", "0x8c2981", "0x8e2a81", "0x902a81", "0x912b81", "0x932b80", "0x942c80", "0x962c80", "0x982d80", "0x992d80", "0x9b2e7f", "0x9c2e7f", "0x9e2f7f", "0xa02f7f", "0xa1307e", "0xa3307e", "0xa5317e", "0xa6317d", "0xa8327d", "0xaa337d", "0xab337c", "0xad347c", "0xae347b", "0xb0357b", "0xb2357b", "0xb3367a", "0xb5367a", "0xb73779", "0xb83779", "0xba3878", "0xbc3978", "0xbd3977", "0xbf3a77", "0xc03a76", "0xc23b75", "0xc43c75", "0xc53c74", "0xc73d73", "0xc83e73", "0xca3e72", "0xcc3f71", "0xcd4071", "0xcf4070", "0xd0416f", "0xd2426f", "0xd3436e", "0xd5446d", "0xd6456c", "0xd8456c", "0xd9466b", "0xdb476a", "0xdc4869", "0xde4968", "0xdf4a68", "0xe04c67", "0xe24d66", "0xe34e65", "0xe44f64", "0xe55064", "0xe75263", "0xe85362", "0xe95462", "0xea5661", "0xeb5760", "0xec5860", "0xed5a5f", "0xee5b5e", "0xef5d5e", "0xf05f5e", "0xf1605d", "0xf2625d", "0xf2645c", "0xf3655c", "0xf4675c", "0xf4695c", "0xf56b5c", "0xf66c5c", "0xf66e5c", "0xf7705c", "0xf7725c", "0xf8745c", "0xf8765c", "0xf9785d", "0xf9795d", "0xf97b5d", "0xfa7d5e", "0xfa7f5e", "0xfa815f", "0xfb835f", "0xfb8560", "0xfb8761", "0xfc8961", "0xfc8a62", "0xfc8c63", "0xfc8e64", "0xfc9065", "0xfd9266", "0xfd9467", "0xfd9668", "0xfd9869", "0xfd9a6a", "0xfd9b6b", "0xfe9d6c", "0xfe9f6d", "0xfea16e", "0xfea36f", "0xfea571", "0xfea772", "0xfea973", "0xfeaa74", "0xfeac76", "0xfeae77", "0xfeb078", "0xfeb27a", "0xfeb47b", "0xfeb67c", "0xfeb77e", "0xfeb97f", "0xfebb81", "0xfebd82", "0xfebf84", "0xfec185", "0xfec287", "0xfec488", "0xfec68a", "0xfec88c", "0xfeca8d", "0xfecc8f", "0xfecd90", "0xfecf92", "0xfed194", "0xfed395", "0xfed597", "0xfed799", "0xfed89a", "0xfdda9c", "0xfddc9e", "0xfddea0", "0xfde0a1", "0xfde2a3", "0xfde3a5", "0xfde5a7", "0xfde7a9", "0xfde9aa", "0xfdebac", "0xfcecae", "0xfceeb0", "0xfcf0b2", "0xfcf2b4", "0xfcf4b6", "0xfcf6b8", "0xfcf7b9", "0xfcf9bb", "0xfcfbbd", "0xfcfdbf"];

        var nodecolorT = [];

        for (var j = 0; j < tmpArr.length; j++) {

            var nodeUI = graphics.getNodeUI(tmpArr[j].key);
            var colorccn = colorM[toNormalize[j]];

            nodecolorT.push({
                key: tmpArr[j].key,
                val: colorccn
            });

            if (nodeUI != null) {
                nodeUI.color = colorccn;
            }
        }

        for (var j = 0; j < tmpArr.length; j++) {
            graph.forEachLinkedNode(tmpArr[j].key, function (linkedNode, link) {

                var links = link.data;

                if (links.startsWith(tmpArr[j].key) === true) {

                    var linkUI = graphics.getLinkUI(link.id);
                    var a = 0;
                    var b = 0;
                    for (var k = 0; k < nodecolorT.length; k++) {
                        if (nodecolorT[k].key === linkedNode.id) {
                            a = nodecolorT[k].val;
                        }
                        if (nodecolorT[k].key === tmpArr[j].key) {
                            b = nodecolorT[k].val;
                        }

                    }

                    var color1 = a.replace(/^.{2}/g, '#');
                    var color2 = b.replace(/^.{2}/g, '#');

                    var percentage = 0.5;
                    var mixed = blend_colors(color1, color2, percentage);

                    var colorl = parseColor(mixed);

                    if (linkUI != null) {
                        linkUI.color = colorl;

                    }
                }
            });
        }

        var t1 = performance.now();
        document.getElementById("ctxt").innerHTML += "(" + (t1 - t0).toFixed(0) + "ms)";
    }

}


function chw(graph, graphics) {

    document.getElementById("in").innerHTML = "";
    document.getElementById("out").innerHTML = "";
    document.getElementById("nodestath").innerHTML = "";
    document.getElementById("scale").style.display = "block";
    document.getElementById("ctxt").innerHTML = "snapshot: chinese whispers (c >= 6)";
    var t0 = performance.now();
    var whisper = createChineseWhisper(graph);

    while (whisper.getChangeRate() > 0) {
        whisper.step();
        //console.log("step");
    }

    // clusters is a Set object
    var clusters = whisper.createClusterMap();

    clusters.forEach(visitCluster);

    function visitCluster(clusterNodes, clusterClass) {
        // each cluster has class identifier:
        console.assert(typeof clusterClass !== undefined);

        // And list of nodes within cluster:
        console.assert(Array.isArray(clusterNodes));
    }

    //console.table(clusters);
    //console.log(clusters.size);
    var it = 50;
    var colorM = ["0x000004", "0x010005", "0x010106", "0x010108", "0x020109", "0x02020b", "0x02020d", "0x03030f", "0x030312", "0x040414", "0x050416", "0x060518", "0x06051a", "0x07061c", "0x08071e", "0x090720", "0x0a0822", "0x0b0924", "0x0c0926", "0x0d0a29", "0x0e0b2b", "0x100b2d", "0x110c2f", "0x120d31", "0x130d34", "0x140e36", "0x150e38", "0x160f3b", "0x180f3d", "0x19103f", "0x1a1042", "0x1c1044", "0x1d1147", "0x1e1149", "0x20114b", "0x21114e", "0x221150", "0x241253", "0x251255", "0x271258", "0x29115a", "0x2a115c", "0x2c115f", "0x2d1161", "0x2f1163", "0x311165", "0x331067", "0x341069", "0x36106b", "0x38106c", "0x390f6e", "0x3b0f70", "0x3d0f71", "0x3f0f72", "0x400f74", "0x420f75", "0x440f76", "0x451077", "0x471078", "0x491078", "0x4a1079", "0x4c117a", "0x4e117b", "0x4f127b", "0x51127c", "0x52137c", "0x54137d", "0x56147d", "0x57157e", "0x59157e", "0x5a167e", "0x5c167f", "0x5d177f", "0x5f187f", "0x601880", "0x621980", "0x641a80", "0x651a80", "0x671b80", "0x681c81", "0x6a1c81", "0x6b1d81", "0x6d1d81", "0x6e1e81", "0x701f81", "0x721f81", "0x732081", "0x752181", "0x762181", "0x782281", "0x792282", "0x7b2382", "0x7c2382", "0x7e2482", "0x802582", "0x812581", "0x832681", "0x842681", "0x862781", "0x882781", "0x892881", "0x8b2981", "0x8c2981", "0x8e2a81", "0x902a81", "0x912b81", "0x932b80", "0x942c80", "0x962c80", "0x982d80", "0x992d80", "0x9b2e7f", "0x9c2e7f", "0x9e2f7f", "0xa02f7f", "0xa1307e", "0xa3307e", "0xa5317e", "0xa6317d", "0xa8327d", "0xaa337d", "0xab337c", "0xad347c", "0xae347b", "0xb0357b", "0xb2357b", "0xb3367a", "0xb5367a", "0xb73779", "0xb83779", "0xba3878", "0xbc3978", "0xbd3977", "0xbf3a77", "0xc03a76", "0xc23b75", "0xc43c75", "0xc53c74", "0xc73d73", "0xc83e73", "0xca3e72", "0xcc3f71", "0xcd4071", "0xcf4070", "0xd0416f", "0xd2426f", "0xd3436e", "0xd5446d", "0xd6456c", "0xd8456c", "0xd9466b", "0xdb476a", "0xdc4869", "0xde4968", "0xdf4a68", "0xe04c67", "0xe24d66", "0xe34e65", "0xe44f64", "0xe55064", "0xe75263", "0xe85362", "0xe95462", "0xea5661", "0xeb5760", "0xec5860", "0xed5a5f", "0xee5b5e", "0xef5d5e", "0xf05f5e", "0xf1605d", "0xf2625d", "0xf2645c", "0xf3655c", "0xf4675c", "0xf4695c", "0xf56b5c", "0xf66c5c", "0xf66e5c", "0xf7705c", "0xf7725c", "0xf8745c", "0xf8765c", "0xf9785d", "0xf9795d", "0xf97b5d", "0xfa7d5e", "0xfa7f5e", "0xfa815f", "0xfb835f", "0xfb8560", "0xfb8761", "0xfc8961", "0xfc8a62", "0xfc8c63", "0xfc8e64", "0xfc9065", "0xfd9266", "0xfd9467", "0xfd9668", "0xfd9869", "0xfd9a6a", "0xfd9b6b", "0xfe9d6c", "0xfe9f6d", "0xfea16e", "0xfea36f", "0xfea571", "0xfea772", "0xfea973", "0xfeaa74", "0xfeac76", "0xfeae77", "0xfeb078", "0xfeb27a", "0xfeb47b", "0xfeb67c", "0xfeb77e", "0xfeb97f", "0xfebb81", "0xfebd82", "0xfebf84", "0xfec185", "0xfec287", "0xfec488", "0xfec68a", "0xfec88c", "0xfeca8d", "0xfecc8f", "0xfecd90", "0xfecf92", "0xfed194", "0xfed395", "0xfed597", "0xfed799", "0xfed89a", "0xfdda9c", "0xfddc9e", "0xfddea0", "0xfde0a1", "0xfde2a3", "0xfde3a5", "0xfde5a7", "0xfde7a9", "0xfde9aa", "0xfdebac", "0xfcecae", "0xfceeb0", "0xfcf0b2", "0xfcf2b4", "0xfcf4b6", "0xfcf6b8", "0xfcf7b9", "0xfcf9bb", "0xfcfbbd", "0xfcfdbf"];
    var carr = [];

    for (var value of clusters.values()) {
        carr.push(value);
    }

    //console.log(carr.length);

    for (var i = 0; i < carr.length; i++) {

        if (carr[i].length >= 6) {
            it = it + 20;
            //console.log(carr[i]);
            for (var j = 0; j < carr[i].length; j++) {

                var nodeUI = graphics.getNodeUI(carr[i][j]);
                if (nodeUI != null) {
                nodeUI.color = colorM[it];
                }
                graph.forEachLinkedNode(carr[i][j], function (linkedNode, link) {

                    var links = link.data;
                    var index = carr[i].indexOf(linkedNode.id);

                    if (links.startsWith(carr[i][j]) === true && index > -1) {


                        var linkUI = graphics.getLinkUI(link.id);
                        var colore = colorM[it].replace(/^.{2}/g, '#');
                        var colorl = parseColor(colore);
                        if (linkUI != null) {
                            linkUI.color = colorl;
                            //previouslinks.push(link.id);
                        }

                    } else {

                    }

                });

            }
        }
    }



    function createChineseWhisper(graph, kind) {
        var api = {
            step: step,
            getClass: getClass,
            getChangeRate: getChangeRate,
            forEachCluster: forEachCluster,
            createClusterMap: createClusterMap
        };

        var changeRate = 1;
        var classChangesCount = 0;
        var random = Viva.random(42);
        var iterator;
        var classMap = new Map();
        var nodeIds = [];

        initInternalStructures();

        return api;

        function step() {
            classChangesCount = 0;
            iterator.forEach(assignHighestClass);
            changeRate = classChangesCount / nodeIds.length;
        }

        function getChangeRate() {
            return changeRate;
        }

        function getClass(nodeId) {
            return classMap.get(nodeId);
        }

        function initInternalStructures() {
            graph.forEachNode(initNodeClass);
            iterator = Viva.randomIterator(nodeIds, random);
            //()

            function initNodeClass(node) {
                classMap.set(node.id, nodeIds.length);
                nodeIds.push(node.id);
            }
        }

        function assignHighestClass(nodeId) {
            var newLevel = getHighestClassInTheNeighborhoodOf(nodeId);
            var currentLevel = classMap.get(nodeId);
            if (newLevel !== currentLevel) {
                classMap.set(nodeId, newLevel);
                classChangesCount += 1;
            }
        }

        function getHighestClassInTheNeighborhoodOf(nodeId) {
            var seenClasses = new Map();
            var maxClassValue = 0;
            var maxClassName = -1;

            graph.forEachLinkedNode(nodeId, visitNeighbour);

            if (maxClassName === -1) {
                // the node didn't have any neighbours
                return classMap.get(nodeId);
            }

            return maxClassName;

            function visitNeighbour(otherNode, link) {
                if (shouldUpdate(link.toId === nodeId)) {
                    var otherNodeClass = classMap.get(otherNode.id);
                    var counter = seenClasses.get(otherNodeClass) || 0;
                    counter += 1;
                    if (counter > maxClassValue) {
                        maxClassValue = counter;
                        maxClassName = otherNodeClass;
                    }

                    seenClasses.set(otherNodeClass, counter);
                }
            }
        }

        function shouldUpdate(isInLink) {
            if (kind === 'in') return isInLink;
            if (kind === 'out') return !isInLink;
            return true;
        }

        function createClusterMap() {
            var clusters = new Map();

            for (var i = 0; i < nodeIds.length; ++i) {
                var nodeId = nodeIds[i];
                var clusterId = getClass(nodeId);
                var nodesInCluster = clusters.get(clusterId);
                if (nodesInCluster) nodesInCluster.push(nodeId);
                else clusters.set(clusterId, [nodeId]);
            }

            return clusters;
        }

        function forEachCluster(cb) {
            var clusters = createClusterMap();

            clusters.forEach(reportToClient);

            function reportToClient(value, key) {
                cb({
                    class: key,
                    nodes: value
                });
            }
        }


    }
    var t1 = performance.now();
    document.getElementById("ctxt").innerHTML += "(" + (t1 - t0).toFixed(0) + "ms)";

}

function blend_colors(color1, color2, percentage) {


    // check input
    color1 = color1 || '#000000';
    color2 = color2 || '#ffffff';
    percentage = percentage || 0.5;

    if (color1.length == 4)
        color1 = color1[1] + color1[1] + color1[2] + color1[2] + color1[3] + color1[3];
    else
        color1 = color1.substring(1);
    if (color2.length == 4)
        color2 = color2[1] + color2[1] + color2[2] + color2[2] + color2[3] + color2[3];
    else
        color2 = color2.substring(1);

    // 3: we have valid input, convert colors to rgb
    color1 = [parseInt(color1[0] + color1[1], 16), parseInt(color1[2] + color1[3], 16), parseInt(color1[4] + color1[5], 16)];
    color2 = [parseInt(color2[0] + color2[1], 16), parseInt(color2[2] + color2[3], 16), parseInt(color2[4] + color2[5], 16)];

    //console.log('hex -> rgba: c1 => [' + color1.join(', ') + '], c2 => [' + color2.join(', ') + ']');

    // 4: blend
    var color3 = [
        (1 - percentage) * color1[0] + percentage * color2[0],
        (1 - percentage) * color1[1] + percentage * color2[1],
        (1 - percentage) * color1[2] + percentage * color2[2]
    ];

    // 5: convert to hex
    color3 = '#' + int_to_hex(color3[0]) + int_to_hex(color3[1]) + int_to_hex(color3[2]);

    return color3;
}

/*
    convert a Number to a two character hex string
    must round, or we will end up with more digits than expected (2)
    note: can also result in single digit, which will need to be padded with a 0 to the left
    @param: num         => the number to conver to hex
    @returns: string    => the hex representation of the provided number
*/
function int_to_hex(num) {
    var hex = Math.round(num).toString(16);
    if (hex.length == 1)
        hex = '0' + hex;
    return hex;
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

/*### oth func ###*/


function tre() {

    var string = connections[18];
    var conne = graph.getLink(string.substr(1, 16), string.substr(17, 16));
    console.log(conne.id);
    return 12;
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
  