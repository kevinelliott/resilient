import { useState } from 'react';
import { Sidebar } from './components/Sidebar';
import { StatusBar } from './components/StatusBar';
import { VaultView } from './views/VaultView';
import { CommsView } from './views/CommsView';
import { MeshView } from './views/MeshView';
import { DownloadsView } from './views/DownloadsView';
import { SwarmView } from './views/SwarmView';
import { ConfigView } from './views/ConfigView';
import { InstallView } from './views/InstallView';

export type ViewState = 'vault' | 'comms' | 'mesh' | 'downloads' | 'swarm' | 'settings' | 'install';

function App() {
  const [currentView, setCurrentView] = useState<ViewState>('vault');

  return (
    <div className="flex h-screen w-screen overflow-hidden bg-bbs-bg text-bbs-text selection:bg-bbs-amber-dim selection:text-bbs-amber">
      <div className="crt-overlay" />

      <Sidebar currentView={currentView} onViewChange={setCurrentView} />

      <main className="flex-1 flex flex-col relative z-10 glass-panel m-2 ml-0 rounded-sm">
        <div className="flex-1 overflow-y-auto p-4 custom-scrollbar">
          {currentView === 'vault' && <VaultView />}
          {currentView === 'comms' && <CommsView />}
          {currentView === 'mesh' && <MeshView />}
          {currentView === 'downloads' && <DownloadsView />}
          {currentView === 'swarm' && <SwarmView />}
          {currentView === 'settings' && <ConfigView />}
          {currentView === 'install' && <InstallView />}
        </div>

        <StatusBar />
      </main>
    </div>
  );
}

export default App;
