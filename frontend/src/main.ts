import { mount } from 'svelte'
import './style.css'
import './debug-overrides.css'
import App from './App.svelte'

function addDebugLabels(): void {
  document.querySelectorAll('div, section, main, header, footer, aside, article, nav, form').forEach(el => {
    const tag = el.tagName.toLowerCase();
    // Filter out Svelte auto-generated hash classes (s-*)
    const cleanClasses = Array.from(el.classList).filter(c => !c.startsWith('s-'));
    const cls = cleanClasses.length ? '.' + cleanClasses.join('.') : '';
    el.setAttribute('data-debug-label', tag + cls);
  });
}

// Enable debug CSS overlay if flag is set
if (import.meta.env.VITE_DEBUG_CSS) {
  document.body.classList.add('debug-mode');
}

mount(App, {
  target: document.getElementById('app') as Element
})

// Add labels after Svelte mounts (it adds its own hash classes)
if (import.meta.env.VITE_DEBUG_CSS) {
  requestAnimationFrame(() => {
    addDebugLabels();
    // Re-add labels on any DOM changes (Svelte may add more elements)
    const observer = new MutationObserver(addDebugLabels);
    observer.observe(document.getElementById('app')!, { childList: true, subtree: true });
  });
}
