import '@fontsource-variable/inter/opsz.css';
import '../styles/tokens.css';
import '../styles/desktop.css';
import { mount } from 'svelte';
import Installer from './Installer.svelte';

mount(Installer, { target: document.getElementById('app')! });
