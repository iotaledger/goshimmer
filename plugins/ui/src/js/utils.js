function uptimeConverter(seconds) {
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
}