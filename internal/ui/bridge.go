package ui

import (
	"net/http"
	"strings"

	"github.com/runwisp/runwisp-manager/internal/config"
)

// BridgeHandler serves the intentionally dependency-free RunWisp integration.
func BridgeHandler(c config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if !c.Bridge {
			http.NotFound(w, nil)
			return
		}
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		script := bridgeScript
		if c.BridgePosition == "bottom-left" {
			script = strings.Replace(script, "right:16px;bottom:16px", "left:16px;bottom:16px", 1)
		}
		if c.BridgePosition == "top-right" {
			script = strings.Replace(script, "right:16px;bottom:16px", "right:16px;top:16px", 1)
		}
		if c.BridgePosition == "top-left" {
			script = strings.Replace(script, "right:16px;bottom:16px", "left:16px;top:16px", 1)
		}
		_, _ = w.Write([]byte(script))
	}
}

const bridgeScript = `(function(){
if(window.__RWM_BRIDGE__)return; window.__RWM_BRIDGE__=true;
const host=document.createElement('div'), root=host.attachShadow({mode:'open'});
const css=':host{all:initial}.pill{position:fixed;z-index:2147483647;right:16px;bottom:16px;display:flex;gap:8px;align-items:center;padding:8px 12px;border:1px solid var(--runwisp-border,#cbd5e1);border-radius:999px;background:var(--runwisp-surface,#fff);color:var(--runwisp-text,#0f172a);box-shadow:0 4px 18px #0002;font:13px system-ui,sans-serif}.pill a,.pill button{color:inherit;background:none;border:0;text-decoration:none;cursor:pointer;font:inherit}.pill button{padding:0}.sep{opacity:.4}';
root.innerHTML='<style>'+css+'</style><nav class="pill" aria-label="RunWisp Manager"></nav>';document.documentElement.appendChild(host);
const nav=root.querySelector('nav'), link=(text,href)=>{const a=document.createElement('a');a.textContent=text;a.href=href;return a};
const render=(me)=>{nav.replaceChildren();nav.append(link('Manager','/manager/'));const match=location.pathname.match(/^\/tasks\/([^/]+)/);if(match&&['editor','admin'].includes(me.role)){nav.append(document.createTextNode(' · '));nav.append(link('Edit task','/manager/tasks/'+encodeURIComponent(match[1])+'/edit'));}nav.append(document.createTextNode(' · '));const user=document.createElement('span');user.textContent=me.name||me.username||me.role;nav.append(user);nav.append(document.createTextNode(' · '));const out=document.createElement('button');out.textContent='Sign out';out.onclick=()=>fetch('/manager/auth/logout',{method:'POST',headers:{'Content-Type':'application/json'}}).then(()=>location.href='/manager/auth/login');nav.append(out)};
fetch('/manager/api/me',{credentials:'same-origin'}).then(r=>{if(r.status===401){location.href='/manager/auth/login?return_to='+encodeURIComponent(location.href);throw Error('unauthenticated')}return r.json()}).then(render).catch(()=>{});
for(const name of ['pushState','replaceState']){const original=history[name];history[name]=function(){const result=original.apply(this,arguments);window.dispatchEvent(new Event('rwm:navigate'));return result}}window.addEventListener('popstate',()=>location.reload());window.addEventListener('rwm:navigate',()=>location.reload());
})();
`
