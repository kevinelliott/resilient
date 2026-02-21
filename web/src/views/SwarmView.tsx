import { Terminal, Download, HardDrive, Network, Plus, ShieldAlert } from 'lucide-react';
import { useState, useEffect } from 'react';
import { CopyableID } from '../components/CopyableID';

type ActiveDownload = {
    file_id: string;
    file_name: string;
    total_chunks: number;
    downloaded_chunks: number;
    status: string; // "downloading", "completed", "failed"
    start_time: number;
    peers: string[];
};

export function SwarmView() {
    const [downloads, setDownloads] = useState<Record<string, ActiveDownload>>({});
    const [peerPopoverID, setPeerPopoverID] = useState<string | null>(null);
    const [peerDb, setPeerDb] = useState<Record<string, string>>({});

    const [showAddPeer, setShowAddPeer] = useState(false);
    const [peerAddress, setPeerAddress] = useState('');
    const [peerStatus, setPeerStatus] = useState<'idle' | 'connecting' | 'success' | 'error'>('idle');
    const [peerStatusMessage, setPeerStatusMessage] = useState('');

    const handleAddPeer = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!peerAddress.trim()) return;
        setPeerStatus('connecting');
        setPeerStatusMessage('Dialing remote peer...');
        try {
            const res = await fetch('/api/peers/connect', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ multiaddr: peerAddress.trim() }),
            });
            if (res.ok) {
                const data = await res.json();
                setPeerStatus('success');
                setPeerStatusMessage(`Connected to ${data.peer_id || 'peer'}`);
                setPeerAddress('');
                setTimeout(() => {
                    setShowAddPeer(false);
                    setPeerStatus('idle');
                    setPeerStatusMessage('');
                }, 2000);
            } else {
                const errText = await res.text();
                setPeerStatus('error');
                setPeerStatusMessage(errText || 'Connection failed');
            }
        } catch (err: any) {
            setPeerStatus('error');
            setPeerStatusMessage(err.message || 'Network error');
        }
    };

    const fetchDownloads = () => {
        fetch('/api/downloads')
            .then(res => res.json())
            .then(data => setDownloads(data?.swarm || {}))
            .catch(err => console.error("Failed to fetch downloads", err));

        fetch('/api/peers')
            .then(res => res.json())
            .then((data: any[]) => {
                const db: Record<string, string> = {};
                if (Array.isArray(data)) {
                    data.forEach(p => {
                        if (p.name) db[p.id] = p.name;
                    });
                }
                setPeerDb(db);
            })
            .catch(err => console.error("Failed to fetch peers", err));
    };

    useEffect(() => {
        fetchDownloads();
        const interval = setInterval(fetchDownloads, 1000);
        return () => clearInterval(interval);
    }, []);

    const dlArray = Object.values(downloads).sort((a, b) => b.start_time - a.start_time);

    return (
        <div className="flex flex-col h-full animate-in fade-in duration-300">
            <header className="mb-4 flex justify-between items-end border-b border-bbs-panel pb-2">
                <div>
                    <h2 className="text-2xl font-bold uppercase tracking-widest text-bbs-cyan drop-shadow-[0_0_8px_rgba(51,255,255,0.4)] flex items-center space-x-2">
                        <Terminal size={24} />
                        <span>Swarm Monitor</span>
                    </h2>
                    <p className="text-bbs-muted text-sm mt-1">Multi-Peer Payload Resolution</p>
                </div>
                <div className="text-xs text-bbs-muted flex items-center space-x-4">
                    <button
                        onClick={() => setShowAddPeer(true)}
                        className="flex items-center space-x-1 px-3 py-1 bg-black text-bbs-cyan border border-bbs-cyan/50 hover:bg-bbs-cyan/10 hover:border-bbs-cyan rounded-sm transition-colors cursor-pointer"
                    >
                        <Plus size={14} /> <span>Manually Dial Peer</span>
                    </button>
                    <span className="flex items-center space-x-1"><div className="w-2 h-2 bg-bbs-green rounded-full shadow-[0_0_5px_var(--color-bbs-green)]" /> <span>CAS Active</span></span>
                </div>
            </header>

            {showAddPeer && (
                <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
                    <div className="bg-black border border-bbs-cyan w-full max-w-lg p-6 rounded-sm shadow-[0_0_30px_rgba(51,255,255,0.15)] animate-in zoom-in-95 duration-200">
                        <div className="flex justify-between items-start mb-6">
                            <h3 className="text-xl font-bold uppercase tracking-widest text-bbs-cyan drop-shadow-[0_0_8px_rgba(51,255,255,0.4)] flex items-center space-x-2">
                                <Network size={20} />
                                <span>Dial Remote Node</span>
                            </h3>
                            <button
                                onClick={() => { setShowAddPeer(false); setPeerStatus('idle'); setPeerStatusMessage(''); }}
                                className="text-bbs-muted hover:text-white bg-transparent border-none text-xl p-0 cursor-pointer"
                            >
                                ×
                            </button>
                        </div>

                        <form onSubmit={handleAddPeer} className="space-y-4">
                            <div>
                                <label className="block text-xs font-bold text-bbs-amber mb-2 uppercase tracking-wide">Peer Multiaddress</label>
                                <input
                                    type="text"
                                    value={peerAddress}
                                    onChange={e => setPeerAddress(e.target.value)}
                                    placeholder="/ip4/192.168.x.x/tcp/4001/p2p/12D3..."
                                    className="w-full bg-black border border-bbs-panel text-bbs-text p-3 font-mono text-sm focus:border-bbs-cyan focus:outline-none transition-colors"
                                    autoFocus
                                />
                                <p className="text-xs text-bbs-muted mt-2">Enter the full Libp2p Multiaddress of the remote hub to establish a direct connection.</p>
                            </div>

                            {peerStatusMessage && (
                                <div className={`p-3 text-sm flex items-center space-x-2 border rounded-sm ${peerStatus === 'error' ? 'text-bbs-red border-bbs-red/30 bg-bbs-red/10' : peerStatus === 'success' ? 'text-bbs-green border-bbs-green/30 bg-bbs-green/10' : 'text-bbs-cyan border-bbs-cyan/30 bg-bbs-cyan/10'}`}>
                                    {peerStatus === 'error' && <ShieldAlert size={14} className="shrink-0" />}
                                    <span className="break-all">{peerStatusMessage}</span>
                                </div>
                            )}

                            <div className="pt-2 flex justify-end space-x-3">
                                <button
                                    type="button"
                                    onClick={() => { setShowAddPeer(false); setPeerStatus('idle'); setPeerStatusMessage(''); }}
                                    className="px-4 py-2 border border-bbs-panel text-bbs-muted hover:text-white hover:border-bbs-text bg-transparent transition-colors text-sm uppercase tracking-wide cursor-pointer"
                                >
                                    Cancel
                                </button>
                                <button
                                    type="submit"
                                    disabled={!peerAddress.trim() || peerStatus === 'connecting'}
                                    className="px-4 py-2 bg-bbs-cyan/10 border border-bbs-cyan text-bbs-cyan font-bold hover:bg-bbs-cyan hover:text-black transition-all text-sm uppercase tracking-wide cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
                                >
                                    {peerStatus === 'connecting' ? 'Dialing...' : 'Connect'}
                                </button>
                            </div>
                        </form>
                    </div>
                </div>
            )}

            <div className="flex-1 overflow-y-auto space-y-4 pr-2 custom-scrollbar">
                {dlArray.length === 0 && (
                    <div className="text-bbs-muted text-sm italic text-center mt-12 bg-black/40 p-8 border border-bbs-panel/50 rounded-sm">
                        No active swarm resolutions detected in local airspace.
                    </div>
                )}

                {dlArray.map(dl => {
                    const progress = dl.total_chunks > 0 ? (dl.downloaded_chunks / dl.total_chunks) * 100 : 0;

                    let statusColor = 'text-bbs-amber';
                    let bgStatusColor = 'bg-bbs-amber';
                    if (dl.status === 'completed') {
                        statusColor = 'text-bbs-green';
                        bgStatusColor = 'bg-bbs-green';
                    } else if (dl.status === 'failed') {
                        statusColor = 'text-bbs-red';
                        bgStatusColor = 'bg-bbs-red';
                    }

                    return (
                        <div key={dl.file_id} className="border border-bbs-panel bg-bbs-surface/50 p-4 rounded-sm relative group">
                            <div className="absolute inset-0 bg-gradient-to-r from-bbs-panel/10 to-transparent pointer-events-none rounded-sm" />

                            <div className="flex justify-between items-start mb-2 relative z-10">
                                <div className="flex-1 min-w-0 pr-4">
                                    <h3 className="text-lg font-bold text-[#e0e0e0] flex items-center space-x-2 truncate hover:text-white transition-colors">
                                        <Download size={16} className={`${statusColor} shrink-0`} />
                                        <span className="truncate">{dl.file_name || 'Virtual Payload'}</span>
                                    </h3>
                                    <div className="mt-1">
                                        <CopyableID label="ID" value={dl.file_id} />
                                    </div>
                                </div>
                                <span className={`text-[10px] font-bold uppercase tracking-wider px-2 py-1 border rounded-sm bg-black ${statusColor} border-current`}>
                                    {dl.status}
                                </span>
                            </div>

                            <div className="mt-4 space-y-2 relative z-10">
                                <div className="flex justify-between text-xs text-bbs-muted relative">
                                    <span className="flex items-center space-x-1">
                                        <HardDrive size={12} /> <span>Chunks: {dl.downloaded_chunks} / {dl.total_chunks}</span>
                                    </span>

                                    <button
                                        onClick={() => setPeerPopoverID(peerPopoverID === dl.file_id ? null : dl.file_id)}
                                        className="flex items-center space-x-1 text-bbs-cyan opacity-80 hover:opacity-100 transition-opacity bg-transparent border-none p-0 cursor-pointer"
                                    >
                                        <Network size={12} /> <span>{Array.isArray(dl.peers) ? dl.peers.length : (typeof dl.peers === 'number' ? dl.peers : 0)} Connected Nodes</span>
                                    </button>

                                    {peerPopoverID === dl.file_id && (
                                        <div className="absolute right-0 top-6 w-64 bg-black border border-bbs-cyan text-bbs-text p-3 rounded-sm z-50 shadow-[0_0_15px_var(--color-bbs-cyan-dim)]">
                                            <h4 className="border-b border-bbs-panel pb-1 mb-2 font-bold text-bbs-cyan uppercase text-[10px] tracking-wider">Active Peers</h4>
                                            <ul className="space-y-1 max-h-40 overflow-y-auto custom-scrollbar font-mono text-[10px]">
                                                {Array.isArray(dl.peers) && dl.peers.length > 0 ? (
                                                    dl.peers.map((p, i) => {
                                                        const pFormat = p.length > 12 ? p.substring(0, 6) + '...' + p.substring(p.length - 4) : p;
                                                        const pDisplay = peerDb[p] ? <><span className="text-bbs-amber font-bold">{peerDb[p]}</span> <span className="opacity-50">({pFormat})</span></> : p;
                                                        return (
                                                            <li key={i} className="truncate text-bbs-muted hover:text-white pointer-events-none" title={p}>{pDisplay}</li>
                                                        );
                                                    })
                                                ) : (
                                                    <li className="text-bbs-red italic">No peer details available.</li>
                                                )}
                                            </ul>
                                        </div>
                                    )}
                                </div>
                                <div className="w-full bg-black h-2 rounded-sm border border-bbs-panel overflow-hidden">
                                    <div
                                        className={`h-full ${bgStatusColor} transition-all duration-500 shadow-[0_0_10px_currentColor]`}
                                        style={{ width: `${progress}%` }}
                                    />
                                </div>
                            </div>
                        </div>
                    );
                })}
            </div>
        </div>
    );
}
