import { Settings, HardDrive, Radio, Shield, Save, Map as MapIcon } from 'lucide-react';
import { useState, useEffect } from 'react';

type ConfigState = {
    node_profile: string;
    cas_limit_gb: number;
    lora_port: string;
    lora_baud: number;
    ble_enabled: boolean;
    auto_zoom_delay_secs: number;
    node_name: string;
};

export function ConfigView() {
    const [config, setConfig] = useState<ConfigState>({
        node_profile: 'hub',
        cas_limit_gb: 50,
        lora_port: '/dev/ttyUSB0',
        lora_baud: 115200,
        ble_enabled: true,
        auto_zoom_delay_secs: 60,
        node_name: ''
    });
    const [isSaving, setIsSaving] = useState(false);
    const [saveMessage, setSaveMessage] = useState<string | null>(null);

    useEffect(() => {
        fetch('/api/config')
            .then(res => res.json())
            .then(data => {
                if (data && Object.keys(data).length > 0) {
                    setConfig(data);
                }
            })
            .catch(err => console.error("Failed to fetch config", err));
    }, []);

    const handleSave = () => {
        setIsSaving(true);
        fetch('/api/config', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(config)
        })
            .then(() => {
                setSaveMessage("CONFIG SAVED");
                setTimeout(() => setSaveMessage(null), 3000);
            })
            .catch(err => {
                console.error("Failed to save config", err);
                setSaveMessage("SAVE ERROR");
            })
            .finally(() => setIsSaving(false));
    };

    return (
        <div className="flex flex-col h-full animate-in fade-in duration-300">
            <header className="mb-6 flex justify-between items-end border-b border-bbs-panel pb-2">
                <div>
                    <h2 className="text-2xl font-bold uppercase tracking-widest text-bbs-cyan drop-shadow-[0_0_8px_var(--color-bbs-cyan-dim)] flex items-center space-x-3">
                        <Settings size={28} />
                        <span>System Config</span>
                    </h2>
                    <p className="text-bbs-muted text-sm mt-1">Core Daemon Parameters & Hardware Interfaces</p>
                </div>
                <button
                    onClick={handleSave}
                    disabled={isSaving}
                    className="bbs-button-cyan flex items-center space-x-2 disabled:opacity-50"
                >
                    <Save size={16} className={isSaving ? "animate-spin" : ""} />
                    <span>{isSaving ? "WRITING..." : "SAVE CONFIG"}</span>
                </button>
            </header>

            {saveMessage && (
                <div className={`mb-4 p-2 text-center font-bold tracking-widest uppercase border ${saveMessage === 'CONFIG SAVED' ? 'bg-bbs-green/10 text-bbs-green border-bbs-green/50' : 'bg-bbs-red/10 text-bbs-red border-bbs-red/50'} animate-in fade-in slide-in-from-top-2`}>
                    {saveMessage}
                </div>
            )}

            <div className="flex-1 overflow-y-auto space-y-6 pr-4 custom-scrollbar">

                {/* Node Profile */}
                <section className="border border-bbs-panel bg-bbs-surface/30 p-4 rounded-sm">
                    <div className="flex items-center space-x-2 mb-4 border-b border-bbs-panel pb-2">
                        <Shield className="text-bbs-amber" size={18} />
                        <h3 className="text-lg font-bold text-bbs-amber uppercase tracking-wider">Node Profile & Identity</h3>
                    </div>

                    <div className="mb-6">
                        <label className="block text-xs uppercase text-bbs-muted mb-1">Node Display Name</label>
                        <input
                            type="text"
                            value={config.node_name}
                            onChange={e => setConfig({ ...config, node_name: e.target.value })}
                            className="w-full bbs-input text-sm font-bold text-bbs-amber"
                            placeholder="ResilientNode"
                            maxLength={64}
                        />
                        <p className="text-xs text-bbs-muted mt-1 italic">Broadcast to peers on connection. Stealth nodes will never share this name.</p>
                    </div>

                    <div className="grid grid-cols-2 gap-4">
                        <div
                            className={`border p-4 cursor-pointer transition-all duration-200 ${config.node_profile === 'hub' ? 'border-bbs-amber bg-bbs-amber/10' : 'border-bbs-panel hover:border-bbs-muted'}`}
                            onClick={() => setConfig({ ...config, node_profile: 'hub' })}
                        >
                            <h4 className="font-bold text-[#e0e0e0] uppercase mb-1">Hub Mode</h4>
                            <p className="text-xs text-bbs-muted leading-relaxed">Aggressive discoverability. Announces to Kademlia DHT, caches remote payloads, and seeds actively across all transports.</p>
                        </div>
                        <div
                            className={`border p-4 cursor-pointer transition-all duration-200 ${config.node_profile === 'stealth' ? 'border-bbs-cyan bg-bbs-cyan/10' : 'border-bbs-panel hover:border-bbs-muted'}`}
                            onClick={() => setConfig({ ...config, node_profile: 'stealth' })}
                        >
                            <h4 className="font-bold text-[#e0e0e0] uppercase mb-1">Stealth Mode</h4>
                            <p className="text-xs text-bbs-muted leading-relaxed">Passive listener. Will not announce presence to DHT or gossipsub. Only downloads chunks on explicit request. Reduces radio footprint.</p>
                        </div>
                    </div>
                </section>

                {/* Storage Limits */}
                <section className="border border-bbs-panel bg-bbs-surface/30 p-4 rounded-sm">
                    <div className="flex items-center space-x-2 mb-4 border-b border-bbs-panel pb-2">
                        <HardDrive className="text-bbs-cyan" size={18} />
                        <h3 className="text-lg font-bold text-bbs-cyan uppercase tracking-wider">CAS Storage Allowance</h3>
                    </div>

                    <div className="space-y-4 max-w-xl">
                        <div>
                            <label className="block text-xs uppercase text-bbs-muted mb-2">Max Chunk Directory Size (GB)</label>
                            <div className="flex items-center space-x-3">
                                <input
                                    type="range"
                                    min="1"
                                    max="500"
                                    value={config.cas_limit_gb}
                                    onChange={e => setConfig({ ...config, cas_limit_gb: parseInt(e.target.value) })}
                                    className="flex-1 accent-bbs-cyan"
                                />
                                <div className="w-20 text-right font-mono text-bbs-cyan font-bold">{config.cas_limit_gb} GB</div>
                            </div>
                            <p className="text-xs text-bbs-muted mt-2 italic">When cache reaches limit, the daemon will prune the least recently accessed chunks (excluding pinned catalogs).</p>
                        </div>
                    </div>
                </section>

                {/* Hardware Bridges */}
                <section className="border border-bbs-panel bg-bbs-surface/30 p-4 rounded-sm">
                    <div className="flex items-center space-x-2 mb-4 border-b border-bbs-panel pb-2">
                        <Radio className="text-bbs-green" size={18} />
                        <h3 className="text-lg font-bold text-bbs-green uppercase tracking-wider">Hardware Radio Bridges</h3>
                    </div>

                    <div className="grid grid-cols-2 gap-6">
                        <div className="space-y-4">
                            <h4 className="text-sm font-bold text-[#e0e0e0] uppercase border-b border-bbs-panel/50 pb-1 mb-2">LoRa Serial Interface</h4>
                            <div>
                                <label className="block text-xs uppercase text-bbs-muted mb-1">Serial Port Path</label>
                                <input
                                    type="text"
                                    value={config.lora_port}
                                    onChange={e => setConfig({ ...config, lora_port: e.target.value })}
                                    className="w-full bbs-input text-sm"
                                    placeholder="/dev/ttyUSB0 or COM3"
                                />
                            </div>
                            <div>
                                <label className="block text-xs uppercase text-bbs-muted mb-1">Baud Rate</label>
                                <select
                                    value={config.lora_baud}
                                    onChange={e => setConfig({ ...config, lora_baud: parseInt(e.target.value) })}
                                    className="w-full bbs-input text-sm bg-black"
                                >
                                    <option value="9600">9600</option>
                                    <option value="38400">38400</option>
                                    <option value="115200">115200</option>
                                </select>
                            </div>
                        </div>

                        <div className="space-y-4">
                            <h4 className="text-sm font-bold text-[#e0e0e0] uppercase border-b border-bbs-panel/50 pb-1 mb-2">Bluetooth Low Energy</h4>
                            <div>
                                <label className="flex items-center space-x-3 cursor-pointer mt-6">
                                    <input
                                        type="checkbox"
                                        checked={config.ble_enabled}
                                        onChange={e => setConfig({ ...config, ble_enabled: e.target.checked })}
                                        className="w-4 h-4 accent-bbs-green bg-black border-bbs-panel"
                                    />
                                    <span className="text-sm uppercase text-[#e0e0e0]">Enable BLE Broadcasting</span>
                                </label>
                                <p className="text-xs text-bbs-muted mt-2 leading-relaxed">Broadcasts the node's Wi-Fi credentials out-of-band to mobile PWA clients for effortless direct connection handoffs.</p>
                            </div>
                        </div>
                    </div>
                </section>

                {/* Mesh Topology Behavior */}
                <section className="border border-bbs-panel bg-bbs-surface/30 p-4 rounded-sm">
                    <div className="flex items-center space-x-2 mb-4 border-b border-bbs-panel pb-2">
                        <MapIcon className="text-bbs-green" size={18} />
                        <h3 className="text-lg font-bold text-bbs-green uppercase tracking-wider">Mesh Topology Behavior</h3>
                    </div>

                    <div className="space-y-4 max-w-xl">
                        <div>
                            <label className="block text-xs uppercase text-bbs-muted mb-2">Auto-Zoom Interaction Delay (Seconds)</label>
                            <div className="flex items-center space-x-3">
                                <input
                                    type="range"
                                    min="0"
                                    max="300"
                                    step="10"
                                    value={config.auto_zoom_delay_secs}
                                    onChange={e => setConfig({ ...config, auto_zoom_delay_secs: parseInt(e.target.value) })}
                                    className="flex-1 accent-bbs-green"
                                />
                                <div className="w-20 text-right font-mono text-bbs-green font-bold">{config.auto_zoom_delay_secs}s</div>
                            </div>
                            <p className="text-xs text-bbs-muted mt-2 italic">When you manually pan or zoom the Mesh Graph, auto-zoom logic will pause for this duration before snapping back to fit all nodes.</p>
                        </div>
                    </div>
                </section>
            </div>
        </div>
    );
}
