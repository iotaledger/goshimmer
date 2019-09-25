new Vue({
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
})