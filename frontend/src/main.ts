import { mount } from 'svelte'
import { WindowShow } from '@wails/runtime/runtime.js'
import './style.css'
import App from './App.svelte';

mount(App, {
  target: document.getElementById('app') as Element
});

// The window is created hidden (StartHidden) so WebKit's light pre-page fill is
// never mapped. rAF is paused while the window is unmapped, so use a timer long
// enough for the app's first layout; the 3s fail-safe in the Go backend covers
// a slow machine.
setTimeout(() => WindowShow(), 200);
