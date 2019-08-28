
Vue.component('force-graph', {
  props:['neighbors', 'me'],
  data: function() {
    return  {
      hoveredNode: null
    }
  },
  watch:{
    neighbors: function (val, oldVal) { 
      let updateNeeded
      val.forEach(node=> {
        if(!oldVal.find(n=> n.id===node.id)) updateNeeded=true
      })
      oldVal.forEach(node=> {
        if(!val.find(n=> n.id===node.id)) updateNeeded=true
      })
      if (updateNeeded) this.graph.graphData(this.makeGraph())
    },
  },
  methods:{
    makeGraph() {
      return {
        nodes: [{id: this.me}, ...this.neighbors],
        links: this.neighbors.filter(id => id)
          .map(id => ({ source: id, target: this.me }))
      };
    }
  },
  mounted() {
    const el = document.getElementById('3d-graph')
    const parentStyle = window.getComputedStyle(el.parentNode)
    this.graph = ForceGraph3D()(el)
      .width(parseInt(parentStyle.width)-2)
      .height(parseInt(parentStyle.height)-2)
      .enableNodeDrag(false)
      .onNodeHover(node => {
        this.hoveredNode = node
        el.style.cursor = node ? 'pointer' : null
      })
      .onNodeClick(node => console.log(node))
      .nodeColor(node => {
        if (node.id===this.me) return 'rgba(0,200,255,1)'
        return node.accepted ? 'rgba(150,255,150,1)' : 'rgba(255,255,255,1)'
      })
      .linkColor('rgba(255,255,255,1)')
      .linkOpacity(0.85)
      .graphData(this.makeGraph())
  },

  template: '<div style="height:100%;">'+
    '<div id="3d-graph"></div>'+
    '<div v-if="hoveredNode" class="hovered-node">'+
      'ID: {{hoveredNode.id}} &nbsp;&nbsp; '+
      '<span v-if="hoveredNode.address">Address: {{hoveredNode.address}}</span>'+
    '</div>'+
  '</div>'
})