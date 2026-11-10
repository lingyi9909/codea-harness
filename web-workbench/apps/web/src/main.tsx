import React from 'react';
import { createRoot } from 'react-dom/client';
import { WorkbenchApp } from './app.tsx';
import './styles.css';

const container = document.getElementById('root');
if (!container) throw new Error('WEB_ROOT_NOT_FOUND');
createRoot(container).render(<React.StrictMode><WorkbenchApp/></React.StrictMode>);
