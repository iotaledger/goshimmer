package ui

var files = map[string]string{
	"css/styles.css": `/* mobile */
html {
  font-size:12px;
}
/* tablets */
@media only screen and (min-width: 600px) {
  html {
    font-size:14px;
  }
}
/* desktop */
@media only screen and (min-width: 768px) {
  html {
    font-size:16px;
  }
}
body {
  opacity: 1;
  transition: 0.35s opacity;
}
body.fade {
    opacity: 0;
    transition: none;
}

/* bulma */
[v-cloak] {
  display: none;
}
.section {
  padding: 1.8rem 1.5rem; 
}
.columns {
  margin-bottom: 0 !important;
}
.header{
  background:#005050;
  padding-bottom: 0;
}

/* main */
.title{
  display: flex;
  align-items: center;
}
.login{
  background:#004965;
  border: 1px solid #5e6d6f;
  padding:20px;
  font-size: 0.9rem;
  border-radius: 0.4em;
  box-shadow: 0 2px 3px rgba(10, 10, 10, 0.1), 0 0 0 1px rgba(10, 10, 10, 0.1); 
}
.login button{
  width:100%;
  margin-top:12px;
}
.login div {
  display: flex;
  align-items: center;
  font-size:1.2rem;
}
.login div:first-child {
  margin-bottom:12px;
}
.login div span {
  min-width:6.5rem;
}
.status{
  margin-left:54px;
}
.status > span {
  margin-left:8px;
}
.tps.input {
  width:100px;
}
.tx-toolbar {
  display: flex;
  justify-content: space-between;
  height:50px;
}
.graph {
  border: 1px solid #5e6d6f;
  overflow: hidden;
}
#graph {
  width:100%;
  height: 100%;
}
.hovered-node{
  position: absolute;
  top: 0;
  font-size: 0.75rem;
  padding: 5px 10px;
  width: 100%;
  color: #c5c5c5;
}
.chart-container {
  height: calc(100% - 50px);
  display: flex;
  border:1px solid #5e6d6f;
  overflow: hidden;
}

/* tables */
.info-list{
  background:#004965;
  border: 1px solid #5e6d6f;
  font-size: 0.9rem;
}
.info-list .list-item{
  display: flex;
  justify-content: space-between;
  white-space: nowrap;
  overflow: hidden;
}

.logs-container {
  overflow: scroll;
  display: flex;
  flex-direction: column-reverse;
  border:1px solid #5e6d6f;
}
.logs-list {
  background:#004965;
  table-layout: fixed;
  width: 100%;
  font-size: 0.9rem;
}
.logs-list th {
  background: #00374c;
}
.logs-list td {
  font-family: "Inconsolata", "Consolas", "Monaco", monospace;
  padding: 0.25em 0.75em;
  overflow: auto;
  white-space: nowrap;
}
.logs-list td::-webkit-scrollbar { height: 0 !important }
.logs-list td:first-child {
  font-weight: bold;
}
.logs-list td:last-child span {
  width:35px;
  text-align: center;
  font-weight: bold;
  display: inline-block;
}

.tx-container {
  height: 100%;
  overflow: scroll;
  display: flex;
  flex-direction: column-reverse;
  border:1px solid #5e6d6f;
}
.tx-list {
  background:#004965;
  table-layout: fixed;
  width: 100%;
  font-size: 0.9rem;
}
.tx-list th {
  padding-bottom: 3px;
  border-top:1px solid #5e6d6f !important;
  background: #00374c;
}
.tx-list tbody tr {
  cursor: pointer;
}
.tx-list tbody tr:hover {
  background: #00374c;
}
.tx-list td {
  font-family: "Inconsolata", "Consolas", "Monaco", monospace;
  padding: 0.25em 0.75em;
  overflow: auto;
  white-space: nowrap;
}
.tx-list td::-webkit-scrollbar { height: 0 !important }
.tx-list td:first-child {
  font-weight: bold;
}
.tx-list tr.full-tx {
  display:block;
  width:100%;
}
.tx-list tr.full-tx td {
  padding:0;
  width:100%;
}
.tx-list tr.full-tx pre:hover {
  background:#232929;
}
.no-txs{
  height:100%;
  width:100%;
  display:flex;
  align-items: center;
  justify-content: center;
  color:grey;
  font-size: 0.8rem;
}
`,
	"index.html": `<!DOCTYPE html>
<html>

<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <link rel="shortcut icon" href="ui/favicon.ico">
  <title>Shimmer UI</title>
  <link rel="stylesheet" href="https://unpkg.com/bulmaswatch@0.7.5/darkly/bulmaswatch.min.css">
  <link rel="stylesheet" href="ui/css/styles.css">
</head>

<body class="fade">
  <section id="app">
    <section class="section header" ref="header">
      <div class="container">
        <div class="columns">
          <div class="column">
            <h1 class="title">
              <iota-icon size="42"></iota-icon>
              Shimmer
            </h1>
            <span v-if="loggedIn" class="status">Status:<span class="tag is-light">{{synced}}</span>
          </div>

          <div v-if="!loggedIn" class="column">
            <form class="login" @submit="login">
              <div>
                <span>Username:</span>
                <input class="input" placeholder="Username" name="username">
              </div>
              <div>
                <span>Password:</span>
                <input class="input" placeholder="Username" name="password">
              </div>
              <button type="submit" :disabled="loginError?true:false"
                :class="loginError?'button is-danger':'button is-primary'">
                {{ loginButtonText }}
              </button>
            </form>
          </div>

          <div v-if="loggedIn" class="column">
            <div class="list info-list">
              <div v-for="(c, i) in infoKeys" class="list-item">
                <span>{{ c }}:</span>&nbsp;
                <span v-if="infoValues[i]">{{ infoValues[i] }}</span>
              </li>
            </div>
          </div>
        </div>
      </div>
      <div v-if="loggedIn" class="tabs is-boxed">
        <ul>
          <li v-for="t in tabs" @click="selectTab(t)"
            :class="selectedTab===t?'is-active':''">
            <a>{{t}}</a>
          </li>
        </ul>
      </div>
    </section>

    <section class="section" v-if="selectedTab==='Logs'">
      <div class="container logs-container" :style="footerContainerStyle()">
        <table class="table logs-list">
          <thead v-if="logs.length>0"><tr>
            <th style="width:75px;">Time</th>
            <th>Message</th>
            <th style="width:75px;">Status</th>
          </tr></thead>
          <tbody>
            <tr v-for="log in logs">
              <td>{{ log.time }}</td>
              <td>{{ log.source }}: {{ log.message }}</td>
              <td>[<span :style="'color:'+log.color+';'">{{ log.label }}</span>]</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="section" v-if="selectedTab==='Spammer'">
      <div class="container" :style="footerContainerStyle()">
        <div class="tx-toolbar">
          <div class="field is-grouped">
            <p class="control">
              <input ref="tpsinput" class="input tps" type="number" v-model="tpsToSpam" placeholder="TPS">
            </p>
            <p class="control">
              <button class="button is-info" @click="startSpam()" :disabled="tpsToSpam<1">
                <play-icon size="19"></play-icon>
              </button>
            </p>
            <p class="control">
              <button class="button is-primary" @click="stopSpam()" :disabled="info.receivedTps<1">
                <stop-icon size="19"></stop-icon>
              </button>
            </p>
          </div>
          <!-- <div class="field is-grouped">
            <button class="button" @click="clearTxs()" :disabled="txs.length<1">
              Clear
            </button>
          </div> -->
        </div>
        <div class="chart-container">
          <tps-chart ref="tpschart"></tps-chart>
        </div>
      </div>
    </section>

    <section class="section" v-if="selectedTab==='Transactions'">
      <div class="container transactions" :style="footerContainerStyle()">
        <div class="tx-container">
          <table v-if="txs.length>0" class="table tx-list">
            <thead><tr>
              <th style="width:50px;"><iota-icon size="20"></iota-icon></th>
              <th style="line-height:21px;">Hash</th>
            </tr></thead>
            <tbody>
              <tr v-for="tx in txs" @click="selectTxHash(tx.hash)" :class="selectedTxHash===tx.hash ? 'full-tx' : ''">
                <td v-if="selectedTxHash!==tx.hash">{{ tx.value }}</td>
                <td>
                  <span v-if="selectedTxHash!==tx.hash">{{ tx.hash }}</span>
                  <pre v-if="selectedTxHash===tx.hash" :style="'width:calc('+(windowWidth-2)+'px - 3rem);'">{{ JSON.stringify(tx,null,2) }}</pre>
                </td>
              </tr>
            </tbody>
          </table>
          <div v-if="txs.length===0" class="no-txs">No transactions yet</div>
        </div>
      </div>
    </section>

    <section class="section" v-if="selectedTab==='Neighbors'">
      <div class="container graph" :style="footerContainerStyle()">
        <force-graph :neighbors="neighbors" :me="info.id"></force-graph>
      </div>
    </section>

  </section>

  <!-- <script src="https://unpkg.com/vue@2.5.9"></script> -->
  <script src="https://unpkg.com/vue@2.6.10/dist/vue.min.js"></script>
  <script src="https://unpkg.com/dayjs@1.8.15"></script>
  <script src="https://unpkg.com/3d-force-graph"></script>
  <script src="https://code.jquery.com/jquery-3.1.1.min.js"></script>
  <script src="https://code.highcharts.com/stock/highstock.js"></script>
  <script src="https://code.highcharts.com/stock/modules/exporting.js"></script>
  <script src="https://code.highcharts.com/stock/modules/export-data.js"></script>
  <script src="ui/js/initials.js"></script>
  <script src="ui/js/utils.js"></script>
  <script src="ui/js/icons.js"></script>
  <script src="ui/js/forcegraph.js"></script>
  <script src="ui/js/tpschart.js"></script>
  <script src="ui/js/main.js"></script>
</body>

</html>`,
	"js/forcegraph.js": `
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
})`,
	"js/icons.js": `
Vue.component('iota-icon', {
  props:['size'],
  template: '<svg :height="size" :width="size" style="margin-right:10px;" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" version="1.1" id="Layer_1" x="0px" y="0px" viewBox="0 0 128 128" xml:space="preserve"><path d="M79.6 24.9c-.1-2.8 2.4-5.4 5.6-5.3 2.8.1 5.1 2.5 5.1 5.4 0 3-2.5 5.3-5.6 5.3-2.8 0-5.1-2.5-5.1-5.4z" fill="#FFF"/><path d="M91 95.4c3 0 5.3 2.3 5.3 5.3s-2.4 5.4-5.4 5.4c-2.9 0-5.3-2.4-5.3-5.4 0-3 2.4-5.3 5.4-5.3z" fill="#FFF"/><path d="M22.4 73.4c-3 0-5.3-2.4-5.3-5.5 0-2.9 2.4-5.2 5.4-5.2 3 0 5.3 2.4 5.3 5.5-.1 2.9-2.5 5.2-5.4 5.2z" fill="#FFF"/><path d="M81.9 39.2c0-2.6 2-4.6 4.6-4.6 2.5 0 4.6 2.1 4.6 4.6 0 2.5-2.1 4.6-4.6 4.6-2.6 0-4.6-2.1-4.6-4.6z" fill="#FFF"/><path d="M33.9 55.1c2.6 0 4.6 2 4.7 4.5 0 2.5-2.1 4.6-4.6 4.6-2.5 0-4.6-2.1-4.6-4.6-.1-2.5 2-4.5 4.5-4.5z" fill="#FFF"/><path d="M98.4 45.4c-2.5 0-4.6-2.1-4.6-4.6 0-2.6 2.1-4.6 4.6-4.6 2.5 0 4.6 2.1 4.6 4.6 0 2.6-2 4.6-4.6 4.6z" fill="#FFF"/><path d="M77.9 99.5c-2.5 0-4.6-2.1-4.6-4.6 0-2.5 2-4.5 4.6-4.6 2.5 0 4.6 2.1 4.6 4.6 0 2.5-2.1 4.6-4.6 4.6z" fill="#FFF"/><path d="M33.9 48.5c0 2.5-2.1 4.6-4.6 4.6-2.5 0-4.5-2.1-4.5-4.6 0-2.5 2.1-4.6 4.6-4.6 2.5-.1 4.6 2 4.5 4.6z" fill="#FFF"/><path d="M70.4 109c-2.5 0-4.5-2-4.5-4.6 0-2.5 2-4.5 4.6-4.6 2.5 0 4.6 2.1 4.6 4.6-.1 2.6-2.1 4.6-4.7 4.6z" fill="#FFF"/><path d="M56.9 97.1c0-2.2 1.7-3.9 3.9-3.9s3.9 1.8 3.9 4-1.8 3.9-3.9 3.9c-2.2 0-3.9-1.8-3.9-4z" fill="#FFF"/><path d="M100.9 52.9c0 2.2-1.8 3.9-3.9 3.9-2.1 0-3.9-1.8-3.9-3.9 0-2.2 1.8-4 3.9-3.9 2.1 0 3.9 1.7 3.9 3.9z" fill="#FFF"/><path d="M44.4 43.7c0 2.2-1.7 3.9-3.9 3.9s-3.9-1.7-3.9-4c0-2.2 1.7-3.9 3.9-3.9 2.3.1 4 1.8 3.9 4z" fill="#FFF"/><path d="M49 54.9c0 2.2-1.7 3.9-3.9 3.9s-3.9-1.7-3.9-3.9 1.7-4 3.9-3.9c2.2 0 3.9 1.7 3.9 3.9z" fill="#FFF"/><path d="M35 33.5c0-2.2 1.8-3.9 3.9-3.9 2.2 0 3.9 1.8 3.9 3.9 0 2.2-1.7 3.9-3.9 3.9S35 35.7 35 33.5z" fill="#FFF"/><path d="M81.1 51.3c0-2.2 1.7-3.9 3.9-3.9s4 1.8 3.9 4c0 2.2-1.8 3.9-3.9 3.9-2.2-.2-3.9-1.9-3.9-4z" fill="#FFF"/><path d="M68.2 83.7c2.1 0 3.9 1.8 3.9 3.9 0 2.2-1.8 3.9-4 3.9s-3.9-1.7-3.9-3.9c.1-2.2 1.9-3.9 4-3.9z" fill="#FFF"/><path d="M56.7 103.6c0 2.2-1.7 3.9-3.9 3.9s-3.9-1.7-3.9-3.9 1.8-4 3.9-3.9c2.2 0 3.9 1.7 3.9 3.9z" fill="#FFF"/><path d="M106.5 60.5c-2.1 0-3.8-1.8-3.9-3.9 0-2.2 1.8-3.9 4-3.9 2.1 0 3.9 1.8 3.9 3.9 0 2.2-1.8 3.9-4 3.9z" fill="#FFF"/><path d="M57.5 89.3c0 1.9-1.5 3.4-3.4 3.3-1.9 0-3.4-1.5-3.4-3.3 0-1.9 1.5-3.4 3.4-3.4s3.4 1.5 3.4 3.4z" fill="#FFF"/><path d="M50.7 38.5c1.9 0 3.3 1.5 3.3 3.3 0 1.8-1.5 3.4-3.3 3.4-1.8 0-3.4-1.5-3.4-3.4.1-1.8 1.6-3.3 3.4-3.3z" fill="#FFF"/><path d="M58.2 79.7c0-1.9 1.4-3.4 3.3-3.4s3.4 1.5 3.4 3.3c0 1.9-1.5 3.3-3.3 3.3-1.9.1-3.4-1.3-3.4-3.2z" fill="#FFF"/><path d="M46.2 99.1c-1.9 0-3.4-1.4-3.4-3.3s1.5-3.3 3.3-3.4c1.9 0 3.4 1.5 3.4 3.4 0 1.8-1.5 3.3-3.3 3.3z" fill="#FFF"/><path d="M84.7 61c0 1.8-1.4 3.3-3.3 3.3s-3.4-1.5-3.3-3.4c0-1.9 1.5-3.3 3.3-3.3 1.9 0 3.3 1.5 3.3 3.4z" fill="#FFF"/><path d="M49.1 35c-1.9 0-3.4-1.4-3.4-3.3s1.5-3.4 3.4-3.4c1.8 0 3.3 1.5 3.4 3.4 0 1.8-1.5 3.3-3.4 3.3z" fill="#FFF"/><path d="M93.4 66c-1.9 0-3.3-1.5-3.3-3.3 0-1.8 1.5-3.3 3.4-3.3s3.3 1.5 3.3 3.4c0 1.8-1.5 3.2-3.4 3.2z" fill="#FFF"/><path d="M55.2 56.4c-1.8 0-3.3-1.5-3.3-3.4 0-1.8 1.6-3.3 3.4-3.3 1.9 0 3.3 1.5 3.3 3.4s-1.5 3.3-3.4 3.3z" fill="#FFF"/><path d="M103 69.6c-1.9-.1-3.3-1.6-3.3-3.5.1-1.8 1.6-3.3 3.4-3.2 1.9.1 3.4 1.6 3.3 3.6-.1 1.8-1.6 3.2-3.4 3.1z" fill="#FFF"/><path d="M64 56.4c-1.6 0-2.9-1.3-2.9-2.9 0-1.6 1.3-2.8 2.9-2.8 1.6 0 2.9 1.3 2.8 2.9.1 1.6-1.1 2.8-2.8 2.8z" fill="#FFF"/><path d="M95.4 73.7c0-1.6 1.2-2.9 2.8-2.9 1.6 0 2.9 1.2 2.9 2.9 0 1.6-1.4 3-2.9 3-1.5-.1-2.8-1.4-2.8-3z" fill="#FFF"/><path d="M60.7 32.2c0 1.6-1.2 2.8-2.9 2.8-1.6 0-2.9-1.3-2.9-2.9 0-1.6 1.3-2.9 3-2.8 1.6 0 2.8 1.3 2.8 2.9z" fill="#FFF"/><path d="M60.4 71.9c0 1.6-1.2 2.8-2.8 2.9-1.7 0-2.9-1.3-2.9-2.9 0-1.6 1.3-2.9 2.9-2.9 1.6 0 2.8 1.3 2.8 2.9z" fill="#FFF"/><path d="M50.2 84.3c-1.6 0-2.9-1.2-2.9-2.8 0-1.6 1.3-2.9 2.9-2.9 1.5 0 2.8 1.3 2.8 2.8 0 1.6-1.2 2.9-2.8 2.9z" fill="#FFF"/><path d="M91.5 70c0 1.6-1.2 2.9-2.9 2.9-1.5 0-2.9-1.4-2.9-2.9 0-1.6 1.3-2.9 2.9-2.9 1.7 0 2.9 1.3 2.9 2.9z" fill="#FFF"/><path d="M76.7 65.5c1.7 0 2.8 1.2 2.9 2.8 0 1.6-1.2 2.9-2.9 2.9-1.6 0-2.9-1.3-2.9-2.9 0-1.6 1.2-2.8 2.9-2.8z" fill="#FFF"/><path d="M45 87.9c0 1.6-1.3 2.9-2.9 2.9-1.6 0-2.9-1.3-2.9-2.9 0-1.6 1.3-2.9 2.9-2.9 1.6 0 2.9 1.3 2.9 2.9z" fill="#FFF"/><path d="M59.5 45.2c-1.6 0-2.9-1.3-2.9-2.9 0-1.5 1.3-2.9 2.9-2.9 1.5 0 2.9 1.3 2.9 2.9 0 1.6-1.3 2.9-2.9 2.9z" fill="#FFF"/><path d="M85.8 75.1c0 1.4-1.1 2.5-2.5 2.5s-2.4-1.1-2.4-2.5 1.1-2.5 2.4-2.5c1.3.1 2.4 1.2 2.5 2.5z" fill="#FFF"/><path d="M58.2 64.6c0 1.4-1.1 2.5-2.4 2.5-1.3 0-2.5-1.2-2.5-2.5s1.2-2.5 2.5-2.5c1.3.1 2.4 1.2 2.4 2.5z" fill="#FFF"/><path d="M40.4 78.2c1.4 0 2.5 1.1 2.4 2.5 0 1.3-1.2 2.5-2.5 2.5-1.4 0-2.5-1.2-2.5-2.6.1-1.4 1.2-2.4 2.6-2.4z" fill="#FFF"/><path d="M71.2 58c-1.4 0-2.5-1-2.5-2.4s1-2.5 2.5-2.5c1.4 0 2.5 1.1 2.5 2.4 0 1.5-1.1 2.5-2.5 2.5z" fill="#FFF"/><path d="M66.7 41.9c1.4 0 2.4 1.1 2.4 2.5s-1.1 2.5-2.4 2.5c-1.4 0-2.5-1.1-2.5-2.5s1.1-2.5 2.5-2.5z" fill="#FFF"/><path d="M50.7 74.2c0 1.4-1.1 2.5-2.4 2.5-1.3 0-2.5-1.2-2.5-2.6 0-1.4 1.1-2.4 2.5-2.4s2.4 1.1 2.4 2.5z" fill="#FFF"/><path d="M71.3 71.1c1.4 0 2.4 1.1 2.4 2.5S72.6 76 71.3 76c-1.4 0-2.5-1.1-2.5-2.5.1-1.4 1.1-2.4 2.5-2.4z" fill="#FFF"/><path d="M95.3 78.8c0 1.4-1 2.5-2.5 2.5-1.4 0-2.4-1.1-2.4-2.5 0-1.3 1.1-2.5 2.4-2.5 1.4 0 2.5 1.1 2.5 2.5z" fill="#FFF"/><path d="M65.1 31.8c1.4 0 2.4 1 2.4 2.4s-1.1 2.5-2.4 2.5c-1.4 0-2.5-1.1-2.5-2.5s1.1-2.4 2.5-2.4z" fill="#FFF"/><path d="M72.7 37.3c0 1.2-1 2.1-2.2 2.1-1.2 0-2.1-1-2.1-2.1 0-1.2.9-2.1 2.1-2.1 1.3 0 2.2.9 2.2 2.1z" fill="#FFF"/><path d="M38.1 74.2c0-1.2.9-2 2.1-2 1.2 0 2.2 1 2.1 2.2 0 1.1-1 2.1-2.1 2.1-1.2 0-2.1-1-2.1-2.3z" fill="#FFF"/><path d="M89.6 82.1c0 1.2-.9 2.2-2.1 2.2-1.2 0-2.2-1-2.2-2.1 0-1.2 1-2.1 2.2-2.1 1.2-.1 2.1.8 2.1 2z" fill="#FFF"/><path d="M50.3 67.9c0 1.2-1 2.1-2.1 2.1-1.2 0-2.1-1-2.1-2.2 0-1.1 1-2.1 2.1-2.1 1.2 0 2.1 1 2.1 2.2z" fill="#FFF"/><path d="M72.2 49.5c-1.2 0-2.1-.9-2.1-2.1 0-1.2.9-2.1 2.1-2.1 1.2 0 2.1.9 2.1 2.1 0 1.3-.9 2.1-2.1 2.1z" fill="#FFF"/><path d="M77.9 76.3c1.2 0 2.1.9 2.1 2.1 0 1.2-1 2.2-2.2 2.2-1.1 0-2-1-2-2.1 0-1.3.9-2.2 2.1-2.2z" fill="#FFF"/><path d="M43.1 69.1c0 1-.8 1.8-1.8 1.8s-1.9-.9-1.9-1.9c0-1 .9-1.8 1.9-1.8 1.1.1 1.8.8 1.8 1.9z" fill="#FFF"/><path d="M84.2 83.8c0 1-.8 1.8-1.8 1.8s-1.8-.9-1.8-1.8c0-1 .8-1.8 1.9-1.8.9 0 1.7.8 1.7 1.8z" fill="#FFF"/><path d="M74.5 39.1c1.1 0 1.9.7 1.9 1.8 0 1-.8 1.8-1.8 1.8s-1.8-.7-1.8-1.8c0-1 .7-1.8 1.7-1.8z" fill="#FFF"/></svg>'
})
Vue.component('play-circle-icon', {
  props:['size'],
  template: '<svg :height="size" :width="size" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512"><path d="M256 464c114.875 0 208-93.125 208-208S370.875 48 256 48 48 141.125 48 256s93.125 208 208 208zm-32-112V160l96 96-96 96z"/></svg>'
})
Vue.component('play-icon', {
  props:['size'],
  template: '<svg :height="size" :width="size" fill="#FFF" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512"><path d="M96 52v408l320-204L96 52z"/></svg>'
})
Vue.component('stop-icon', {
  props:['size'],
  template: '<svg :height="size" :width="size" fill="#FFF" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512"><path d="M405.333 64H106.667C83.198 64 64 83.198 64 106.667v298.666C64 428.802 83.198 448 106.667 448h298.666C428.802 448 448 428.802 448 405.333V106.667C448 83.198 428.802 64 405.333 64z"/></svg>'
})
Vue.component('empty-icon', {
  template: '<svg></svg>'
})`,
	"js/initials.js": `var initialData = {
  loggedIn: false,
  connected: false,
  tabs: ['Logs', 'Spammer', 'Transactions', 'Neighbors'],
  selectedTab: '',
  logs: [],
  tpsToSpam: 1,
  receivedTps:0,
  txs: [],
  info: {},
  selectedTxHash:null,
  tps:0,
  loginError:'',
}
`,
	"js/main.js": `new Vue({
  el: '#app',
  created() {
    this.init()
    window.addEventListener('resize', (e)=>{
      debounce(this.setHeaderSize, 300)
    })
  },
  data: initialData,
  methods: {
    async login(e){
      e.preventDefault()
      const formData = new FormData(e.target);
      const pass = formData.get('password')
      const user = formData.get('username')
      try{
        const r = await api.get('login?username='+user+'&password='+pass)
        if(r.token){
          this.init()
        }
      }catch(e){
        this.loginError = 'Not Authorized'
        setTimeout(()=>this.loginError='',2000)
      }
    },
    async init() {
      try{
        const r = await api.get('loghistory')
        this.updateLogs(r)
      } catch(e) {
        this.show()
        return
      }
      this.show()
      this.loggedIn = true
      this.selectedTab = 'Logs'
      let wsProtocol = 'wss:'
      if (location.protocol != 'https:') {
        wsProtocol = 'ws:'
      }
      this.ws = new WebSocket(
        api.addToken(wsProtocol+'//'+window.location.host+'/ws')
      )
      this.ws.onopen = (e) => { this.connected=true }
      this.ws.onclose = (e) => { this.connected=false }
      this.ws.onerror = (e) => { this.connected=false }
      this.ws.onmessage = (e) => { this.receive(e.data) }
    },
    show() {
      document.body.className = '';
      setTimeout(()=>this.setHeaderSize(),15)
    },
    footerContainerStyle() {
      const size = Math.max(window.innerHeight-this.headerSize, 300)
      return 'height:calc('+(size-1)+'px - 3.6rem);'
    },
    startSpam() {
      console.log('start spam', this.tpsToSpam)
      api.get('spammer?cmd=start&tps='+this.tpsToSpam)
    },
    stopSpam() {
      console.log('stop spam')
      api.get('spammer?cmd=stop')
    },
    selectTab(tab) { 
      if(tab==='Spammer'){
        setTimeout(()=> this.$refs.tpsinput.focus(), 1)
      }
      this.selectedTab=tab
    },
    selectTxHash(hash) {
      if(hash===this.selectedTxHash){
        this.selectedTxHash = null
      } else {
        this.selectedTxHash = hash
      } 
    },
    receive(data) {
      let j
      try {
        j = JSON.parse(data)
      } catch(err){}
      this.parseResponse(j)
    },
    send(data) {
      this.ws.send(JSON.stringify(data))
    },
    parseResponse(r) {
      if(r.info){ // node info
        this.info = r.info
        if(this.$refs.tpschart){
          this.$refs.tpschart.addPoint(r.info.receivedTps)
        }
      }
      if(r.txs) {
        this.updateTxs(r.txs)
      }
      if(r.logs) {
        this.updateLogs(r.logs)
      }
    },
    updateLogs(newLogs){
      this.logs = this.logs.concat(
        newLogs.map(j=>{
          return {
            message:j.message, 
            source:j.source,
            label:logLevels[j.level].label, 
            color:logLevels[j.level].color,
            time: dayjs(j.time).format('hh:mm:ss')}
        })
      )
    },
    updateTxs(tx) {
      const txs = this.txs.concat(tx).reverse()
      this.txs = txs.splice(0,1000).reverse()
    },
    clearTxs() {
      this.txs = []
    },
    setHeaderSize() {
      const h = window.getComputedStyle(this.$refs.header).height
      this.headerSize = parseInt(h)
      this.windowWidth = window.innerWidth
    },
  },
  computed: {
    synced() { return this.connected ? 'Synced':'......' },
    infoKeys() {
      return ['TPS', 'Node ID', 'Neighbors', 'Peers', 'Uptime']
    },
    loginButtonText() {
      return this.loginError || 'Login'
    },
    infoValues() {
      const i = this.info
      return i.id ? [
        i.receivedTps+' received / '+i.solidTps+ 'new',
        i.id,
        i.chosenNeighbors.length+' chosen / '+i.acceptedNeighbors.length+' accepted',
        i.knownPeers+' total / '+i.neighborhood+' neighborhood',
        uptimeConverter(i.uptime)
      ] : Array(5).fill('...')
    },
    neighbors() {
      if(this.info.chosenNeighbors && this.info.acceptedNeighbors) {
        const cn = this.info.chosenNeighbors.map(s=>{
          const a = s.split(' / ')
          return {id: a[1]||'', address: a[0], accepted: false}
        })
        const an = this.info.acceptedNeighbors.map(s=>{
          const a = s.split(' / ')
          return {id: a[1]||'', address: a[0], accepted: true}
        })
        return cn.concat(an)
      }
      return []
    }
  },
})`,
	"js/tpschart.js": `Vue.component('tps-chart', {
  props: ['tps'],
  watch:{
    tps: function (val, oldVal) { 
      console.log(val)
    }
  },
  methods: {
    addPoint(value) {
      var time = Date.now() - 1000;
      this.chart.series[0].addPoint([time, value], true);
    },
  },
  async created() {
    const tpsqueue = await api.get('tpsqueue')
    var time = Date.now() - 1000 * (tpsqueue.length + 1);
    for (let i = 0; i < tpsqueue.length; i++) {
      this.chart.series[0].addPoint([time += 1000, tpsqueue[i]], false);
    }
    this.chart.redraw();
  },
  mounted() {
    Highcharts.createElement('link', {
      href: 'https://fonts.googleapis.com/css?family=Unica+One',
      rel: 'stylesheet',
      type: 'text/css'
    }, null, document.getElementsByTagName('head')[0]);
    Highcharts.theme = highchartsTheme
    Highcharts.setOptions(Highcharts.theme);
    // Start Here
    var time = Date.now() - 3000;

    data = [[time - 5000, 0]];
    var tzoffset = new Date().getTimezoneOffset();
    this.chart = Highcharts.stockChart('chart', {
      rangeSelector: {
        selected: 1
      },
      title: {
        text: 'Transactions per second'
      },
      time: {
        timezoneOffset: tzoffset
      },
      rangeSelector: {
        buttons: [
          {
            type: 'minute',
            count: 5,
            text: '5m'
          }, {
            type: 'minute',
            count: 15,
            text: '15m'
          }, {
            type: 'minute',
            count: 30,
            text: '30m'
          }, {
            type: 'hour',
            count: 60,
            text: '1h'
          }],
        inputEnabled: false
      },
      series: [{
        name: 'Transactions per second',
        data: data,
        type: 'areaspline',
        threshold: null,
        tooltip: {
          valueDecimals: 0
        },
        fillColor: {
          linearGradient: {
            x1: 0,
            y1: 0,
            x2: 0,
            y2: 1
          },
          stops: [
            [0, Highcharts.getOptions().colors[0]],
            [1, Highcharts.Color(Highcharts.getOptions().colors[0]).setOpacity(0).get('rgba')]
          ]
        }
      }]
    });
  },
  template: '<div id="chart" style="height:100%;min-width:100%"></div>'
})

var highchartsTheme = {
  colors: ['#2b908f', '#90ee7e', '#f45b5b', '#7798BF', '#aaeeee', '#ff0066',
    '#eeaaee', '#55BF3B', '#DF5353', '#7798BF', '#aaeeee'],
  chart: {
    backgroundColor: {
      linearGradient: { x1: 0, y1: 0, x2: 1, y2: 1 },
      stops: [
        [0, '#2a2a2b'],
        [1, '#3e3e40']
      ]
    },
    // style: {
    //     fontFamily: 'sans-serif'
    // },
    plotBorderColor: '#606063'
  },
  title: {
    style: {
      color: '#E0E0E3',
      // textTransform: 'uppercase',
      fontSize: '20px'
    }
  },
  subtitle: {
    style: {
      color: '#E0E0E3',
      // textTransform: 'uppercase'
    }
  },
  xAxis: {
    gridLineColor: '#707073',
    labels: {
      style: {
        color: '#E0E0E3'
      }
    },
    lineColor: '#707073',
    minorGridLineColor: '#505053',
    tickColor: '#707073',
    title: {
      style: {
        color: '#A0A0A3'
      }
    }
  },
  yAxis: {
    gridLineColor: '#707073',
    labels: {
      style: {
        color: '#E0E0E3'
      }
    },
    lineColor: '#707073',
    minorGridLineColor: '#505053',
    tickColor: '#707073',
    tickWidth: 1,
    title: {
      style: {
        color: '#A0A0A3'
      }
    }
  },
  tooltip: {
    backgroundColor: 'rgba(0, 0, 0, 0.85)',
    style: {
      color: '#F0F0F0'
    }
  },
  plotOptions: {
    series: {
      dataLabels: {
        color: '#B0B0B3'
      },
      marker: {
        lineColor: '#333'
      }
    },
    boxplot: {
      fillColor: '#505053'
    },
    candlestick: {
      lineColor: 'white'
    },
    errorbar: {
      color: 'white'
    }
  },
  legend: {
    itemStyle: {
      color: '#E0E0E3'
    },
    itemHoverStyle: {
      color: '#FFF'
    },
    itemHiddenStyle: {
      color: '#606063'
    }
  },
  credits: {
    style: {
      color: '#666'
    }
  },
  labels: {
    style: {
      color: '#707073'
    }
  },
  drilldown: {
    activeAxisLabelStyle: {
      color: '#F0F0F3'
    },
    activeDataLabelStyle: {
      color: '#F0F0F3'
    }
  },
  navigation: {
    buttonOptions: {
      symbolStroke: '#DDDDDD',
      theme: {
        fill: '#505053'
      }
    }
  },
  // scroll charts
  rangeSelector: {
    buttonTheme: {
      fill: '#505053',
      stroke: '#000000',
      style: {
        color: '#CCC'
      },
      states: {
        hover: {
          fill: '#707073',
          stroke: '#000000',
          style: {
            color: 'white'
          }
        },
        select: {
          fill: '#000003',
          stroke: '#000000',
          style: {
            color: 'white'
          }
        }
      }
    },
    inputBoxBorderColor: '#505053',
    inputStyle: {
      backgroundColor: '#333',
      color: 'silver'
    },
    labelStyle: {
      color: 'silver'
    }
  },
  navigator: {
    handles: {
      backgroundColor: '#666',
      borderColor: '#AAA'
    },
    outlineColor: '#CCC',
    maskFill: 'rgba(255,255,255,0.1)',
    series: {
      color: '#7798BF',
      lineColor: '#A6C7ED'
    },
    xAxis: {
      gridLineColor: '#505053'
    }
  },
  scrollbar: {
    barBackgroundColor: '#808083',
    barBorderColor: '#808083',
    buttonArrowColor: '#CCC',
    buttonBackgroundColor: '#606063',
    buttonBorderColor: '#606063',
    rifleColor: '#FFF',
    trackBackgroundColor: '#404043',
    trackBorderColor: '#404043'
  },
  // special colors for some of the
  legendBackgroundColor: 'rgba(0, 0, 0, 0.5)',
  background2: '#505053',
  dataLabelsColor: '#B0B0B3',
  textColor: '#C0C0C0',
  contrastTextColor: '#F0F0F3',
  maskColor: 'rgba(255,255,255,0.3)'
}`,
	"js/utils.js": `function uptimeConverter(seconds) {
  var s = ''
  var hrs = Math.floor(seconds / 3600);
  if (hrs) s += hrs+'h '
  seconds -= hrs*3600;
  var mnts = Math.floor(seconds / 60);
  if (mnts) s += mnts+'m '
  seconds -= mnts*60;
  s += seconds + 's'
  return s
}

// indexed by number
const logLevels = [{
  color: '#e74c3c',
  name: 'LOG_LEVEL_FAILURE',
  label: 'FAIL',
},{
  color: '#f1b70e',
  name: 'LOG_LEVEL_WARNING',
  label: 'WARN',
},{
  color: '#2ecc71',
  name: 'LOG_LEVEL_SUCCESS',
  label: 'OK',
},{
  color: '#209cee',
  name: 'LOG_LEVEL_INFO',
  label: 'INFO',
},{
  color: '#375a7f',
  name: 'LOG_LEVEL_DEBUG',
  label: 'NOTE',
}]

const api = {
  get: async function(u) {
    const url = this.trim('/'+u)
    try{
      const r = await fetch(this.addToken(url))
      const j = await r.json()
      if (!(r.status >= 200 && r.status < 300)) {
        throw new Error(j)
      }
      if(url.startsWith('login') && j.token) {
        localStorage.setItem('token', j.token)
        await sleep(1)
      }
      return j
    } catch(e) {
      throw e
    }
  },
  post: async function(u, data){
    const url = this.trim('/'+u)
    try{
      const r = await fetch(this.addToken(url), {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        data: JSON.stringify(data)
      })
      const j = await r.json()
      if (!(r.status >= 200 && r.status < 300)) {
        throw new Error(j)
      }
      return j
    } catch(e) {
      throw e
    }
  },
  trim: function(s) {
    return s.replace(/^\//, '');
  },
  addToken: function(s) {
    const token = localStorage.getItem('token')
    if (!token) return s
    const char = s.includes('?') ? '&' : '?'
    return s + char + 'token=' + token
  }
}

let inDebounce
function debounce(func, delay) {
  const context = this
  const args = arguments
  clearTimeout(inDebounce)
  inDebounce = setTimeout(() => func.apply(context, args), delay)
}

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}`,
}
