import { Activity, Radio, Lock, Wifi, X } from 'lucide-react';
import { useState, useEffect } from 'react';
import { CopyableID } from './CopyableID';

export function StatusBar() {
    const [info, setInfo] = useState<{ status: string; version: string; node_id: string } | null>(null);
    const [error, setError] = useState(false);
    const [peers, setPeers] = useState<{ id: string; status: string; latency: string; route: string; trust: string }[]>([]);
    const [showPeerModal, setShowPeerModal] = useState(false);

    useEffect(() => {
        fetch('/api/info')
            .then(res => res.json())
            .then(data => {
                setInfo(data);
                setError(false);
            })
            .catch(err => {
                console.error("Failed to connect to daemon", err);
                setError(true);
            });
        const fetchPeers = () => {
            fetch('/api/peers')
                .then(res => res.json())
                .then(data => setPeers(data || []))
                .catch(err => console.error("Failed to fetch peers", err));
        };

        fetchPeers();
        const interval = setInterval(fetchPeers, 5000);
        return () => clearInterval(interval);
    }, []);

    return (
        <footer className="h-10 border-t border-bbs-panel bg-bbs-surface/80 flex items-center justify-between px-4 text-xs z-[100] backdrop-blur-sm relative">
            <div className="flex items-center space-x-6 text-bbs-muted">
                <div className="flex items-center space-x-2">
                    {error ? (
                        <>
                            <div className="w-2 h-2 rounded-full bg-bbs-red shadow-[0_0_8px_var(--color-bbs-red)]" />
                            <span className="uppercase text-bbs-red">Daemon Offline</span>
                        </>
                    ) : info ? (
                        <>
                            <div className="w-2 h-2 rounded-full bg-bbs-green animate-pulse shadow-[0_0_8px_var(--color-bbs-green)]" />
                            <span className="uppercase text-bbs-green drop-shadow-[0_0_2px_var(--color-bbs-green)]">Daemon Online</span>
                        </>
                    ) : (
                        <span className="uppercase text-bbs-muted">Connecting...</span>
                    )}
                </div>

                <div className="flex items-center space-x-2 text-bbs-cyan">
                    <Radio size={12} className="opacity-80 drop-shadow-[0_0_3px_var(--color-bbs-cyan)]" />
                    <span className="uppercase tracking-widest">LoRa: Standby</span>
                </div>
            </div>

            <div className="flex items-center space-x-6 text-bbs-muted border-l border-bbs-panel pl-4 py-1 relative">
                <div
                    className="flex items-center space-x-2 group cursor-pointer transition-colors hover:text-bbs-amber"
                    onClick={() => setShowPeerModal(!showPeerModal)}
                >
                    <Activity size={12} className="group-hover:animate-spin" />
                    <span className="uppercase">Peers: <strong className="text-bbs-amber">{peers.length}</strong></span>
                </div>

                {showPeerModal && (
                    <div className="absolute bottom-10 right-0 mb-2 w-96 bg-black border border-bbs-panel rounded-sm shadow-xl flex flex-col max-h-[400px] overflow-hidden animate-in slide-in-from-bottom-2 fade-in">
                        <div className="bg-bbs-panel/30 p-2 text-xs font-bold uppercase tracking-widest text-bbs-cyan border-b border-bbs-panel flex justify-between items-center px-3">
                            <span>Active Routing Table</span>
                            <button onClick={() => setShowPeerModal(false)} className="hover:text-bbs-red transition-colors"><X size={14} /></button>
                        </div>
                        <div className="overflow-y-auto p-2 space-y-2 custom-scrollbar pointer-events-auto">
                            {peers.length === 0 ? (
                                <div className="text-center p-4 text-bbs-muted italic opacity-60">No foreign nodes discovered...</div>
                            ) : (
                                peers.map((p, i) => (
                                    <div key={i} className="p-2 border border-bbs-panel/30 hover:border-bbs-cyan/50 hover:bg-bbs-cyan/5 transition-colors grid grid-cols-2 gap-1 text-[10px]">
                                        <div className="col-span-2 text-bbs-text font-mono truncate text-xs opacity-90 border-b border-bbs-panel/20 pb-1 mb-1">
                                            <CopyableID value={p.id} />
                                        </div>
                                        <div className="flex items-center space-x-1 text-bbs-muted">
                                            <div className={`w-1.5 h-1.5 rounded-full ${p.status === 'Active' ? 'bg-bbs-green shadow-[0_0_3px_var(--color-bbs-green)]' : 'bg-bbs-muted'}`} />
                                            <span className="uppercase">{p.status}</span>
                                        </div>
                                        <div className="text-right text-bbs-muted font-mono">{p.latency}</div>
                                        <div className="flex items-center space-x-1 text-bbs-amber">
                                            {p.route.includes('WiFi') ? <Wifi size={10} /> : <Radio size={10} />}
                                            <span>{p.route}</span>
                                        </div>
                                        <div className="flex justify-end items-center space-x-1 text-bbs-cyan">
                                            <Lock size={10} />
                                            <span className="uppercase">{p.trust}</span>
                                        </div>
                                    </div>
                                ))
                            )}
                        </div>
                    </div>
                )}

                <div className="flex items-center space-x-2 w-48 border-l border-bbs-panel/50 pl-4 ml-2">
                    <Lock size={12} className={info ? "text-bbs-green" : "text-bbs-red"} />
                    {info ? (
                        <CopyableID label="ID" value={info.node_id} />
                    ) : (
                        <span className="uppercase font-mono opacity-80 text-[10px] text-bbs-muted">OFFLINE</span>
                    )}
                </div>
            </div>
        </footer>
    );
}
