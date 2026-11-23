import {StandaloneConfigPage} from './StandaloneConfigPage.tsx';
import {ComponentLabPage} from './ComponentLabPage.tsx';
import {PageAnalyticsPage} from './PageAnalyticsPage.tsx';
import {InstitutionTaskManagementPage} from './InstitutionTaskManagementPage.tsx';
import {AnonymousInstitutionPage} from './AnonymousInstitutionPage.tsx';
import {EmbeddedMessagePage} from './EmbeddedMessagePage.tsx';
import React from 'react';
import { createRoot } from 'react-dom/client';
import { WorkbenchApp } from './app.tsx';
import './styles.css';
import './silver-precision.css';

const container = document.getElementById('root');
if (!container) throw new Error('WEB_ROOT_NOT_FOUND');
createRoot(container).render(<React.StrictMode>{window.location.pathname==='/analytics'?<PageAnalyticsPage/>:window.location.pathname==='/component-lab'?<ComponentLabPage/>:window.location.pathname==='/institution-tasks'?<InstitutionTaskManagementPage/>:window.location.pathname==='/config'?<StandaloneConfigPage/>:window.location.pathname==='/embed/institution-message'?<AnonymousInstitutionPage/>:window.location.pathname==='/embed/message'?<EmbeddedMessagePage/>:<WorkbenchApp/>}</React.StrictMode>);
