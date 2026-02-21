import { Database, MessageSquare, Network, Settings, ShieldAlert, Download, DownloadCloud, Activity } from 'lucide-react';
import { useState, useEffect } from 'react';
import type { ViewState } from '../App';

interface SidebarProps {
    currentView: ViewState;
    onViewChange: (view: ViewState) => void;
}

export function Sidebar({ currentView, onViewChange }: SidebarProps) {
    const navItems: { id: ViewState; label: string; icon: React.ReactNode }[] = [
        { id: 'vault', label: 'Vault', icon: <Database size={20} /> },
        { id: 'comms', label: 'Comms', icon: <MessageSquare size={20} /> },
        { id: 'mesh', label: 'Mesh', icon: <Network size={20} /> },
        { id: 'downloads', label: 'Transfers', icon: <DownloadCloud size={20} /> },
        { id: 'swarm', label: 'Swarm', icon: <Activity size={20} /> },
        { id: 'install', label: 'Install', icon: <Download size={20} /> },
        { id: 'settings', label: 'Config', icon: <Settings size={20} /> },
    ];

    const [uptime, setUptime] = useState("00:00:00");
    const [info, setInfo] = useState<{ node_id?: string, node_name?: string }>({});

    useEffect(() => {
        const fetchUptime = () => {
            fetch('/api/info')
                .then(res => res.json())
                .then(data => {
                    if (data) {
                        setInfo({ node_id: data.node_id, node_name: data.node_name });
                        if (data.uptime_seconds !== undefined) {
                            const h = Math.floor(data.uptime_seconds / 3600).toString().padStart(2, '0');
                            const m = Math.floor((data.uptime_seconds % 3600) / 60).toString().padStart(2, '0');
                            const s = (data.uptime_seconds % 60).toString().padStart(2, '0');
                            setUptime(`${h}:${m}:${s}`);
                        }
                    }
                })
                .catch(err => console.error("Failed to fetch uptime", err));
        };
        fetchUptime();
        const interval = setInterval(fetchUptime, 1000);
        return () => clearInterval(interval);
    }, []);

    return (
        <aside className="w-64 flex flex-col z-10 p-2 border-r border-bbs-panel glass-panel bg-opacity-50 m-2 rounded-sm relative shadow-lg shadow-bbs-amber-dim/5">
            <div className="p-4 mb-8 flex items-center space-x-3 border-b border-bbs-panel pb-6 pt-4">
                <div className="w-10 h-10 rounded-sm bg-bbs-surface border border-bbs-amber flex items-center justify-center text-bbs-amber shadow-[0_0_15px_var(--color-bbs-amber-dim)]">
                    <ShieldAlert size={24} />
                </div>
                <div>
                    <h1 className="text-xl font-bold tracking-widest text-bbs-text uppercase drop-shadow-[0_0_4px_rgba(255,255,255,0.4)] truncate w-40" title={info.node_name || 'Resilient'}>
                        {info.node_name || 'Resilient'}
                    </h1>
                    <div className="text-[10px] text-bbs-amber uppercase tracking-widest opacity-80">
                        {info.node_id ? `${info.node_id.substring(0, 6)}...${info.node_id.substring(info.node_id.length - 4)}` : 'Mesh OS v0.1.0'}
                    </div>
                </div>
            </div>

            <nav className="flex-1 space-y-2 px-2">
                {navItems.map((item) => {
                    const isActive = currentView === item.id;
                    return (
                        <button
                            key={item.id}
                            onClick={() => onViewChange(item.id)}
                            className={`w-full flex items-center space-x-4 px-4 py-3 text-left transition-all duration-200 group relative overflow-hidden ${isActive
                                ? 'bg-bbs-surface text-bbs-amber border-l-2 border-bbs-amber shadow-[inset_0_0_10px_var(--color-bbs-surface)]'
                                : 'text-bbs-muted hover:text-bbs-text hover:bg-bbs-panel/30 border-l-2 border-transparent'
                                }`}
                        >
                            {isActive && (
                                <div className="absolute inset-0 bg-gradient-to-r from-bbs-amber/5 to-transparent pointer-events-none" />
                            )}
                            <span className={`transition-colors duration-200 ${isActive ? 'text-bbs-amber drop-shadow-[0_0_5px_var(--color-bbs-amber-dim)]' : 'group-hover:text-bbs-cyan'}`}>
                                {item.icon}
                            </span>
                            <span className="font-bold uppercase tracking-wider text-sm z-10">{item.label}</span>
                        </button>
                    );
                })}
            </nav>

            <div className="p-4 border-t border-bbs-panel text-xs text-bbs-muted">
                <div className="flex justify-between items-center bg-black/40 p-2 rounded-sm border border-bbs-panel/50">
                    <span className="uppercase">Uptime</span>
                    <span className="text-bbs-cyan">{uptime}</span>
                </div>
            </div>
        </aside>
    );
}
