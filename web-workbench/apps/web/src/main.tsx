/**
 * React 页面启动入口
 *
 * 职责：挂载智能工作台页面，并根据 URL 选择相应应用入口。
 * Java 研发阅读提示：类比 Java 应用启动初始化；这里只负责前端挂载和模块装配。
 *
 * 以下为模块说明注释，不改变原有执行逻辑。
 */
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
