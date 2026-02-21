import { Monitor, Smartphone, Terminal, Download, ShieldCheck, ChevronRight } from 'lucide-react';

export function InstallView() {
    return (
        <div className="flex flex-col h-full animate-in fade-in duration-300">
            <header className="mb-6 flex justify-between items-end border-b border-bbs-panel pb-2">
                <div>
                    <h2 className="text-2xl font-bold uppercase tracking-widest text-bbs-cyan drop-shadow-[0_0_8px_var(--color-bbs-cyan-dim)] flex items-center space-x-3">
                        <Download size={28} />
                        <span>Install Node</span>
                    </h2>
                    <p className="text-bbs-muted text-sm mt-1">Download Native Apps & Daemon Proxies from this Node</p>
                </div>
            </header>

            <div className="flex-1 overflow-y-auto space-y-6 pr-4 custom-scrollbar pb-8">

                {/* Desktop Apps */}
                <section className="border border-bbs-panel bg-bbs-surface/30 p-4 rounded-sm">
                    <div className="flex items-center space-x-2 mb-4 border-b border-bbs-panel pb-2">
                        <Monitor className="text-bbs-cyan" size={18} />
                        <h3 className="text-lg font-bold text-bbs-cyan uppercase tracking-wider">Desktop Applications</h3>
                    </div>

                    <p className="text-sm text-bbs-muted mb-4 leading-relaxed">
                        The full Native GUI is built using extremely lightweight Tauri wrappers utilizing OS-level WebViews. The vault daemon is bundled silently as a background sidecar.
                    </p>

                    <div className="grid grid-cols-3 gap-4">
                        <button className="flex flex-col items-start p-4 border border-bbs-panel hover:border-bbs-cyan transition-colors group">
                            <span className="font-bold text-[#e0e0e0] group-hover:text-bbs-cyan uppercase mb-1">macOS (Apple Silicon)</span>
                            <span className="text-xs text-bbs-muted font-mono">resilient-gui-darwin-arm64.dmg</span>
                        </button>
                        <button className="flex flex-col items-start p-4 border border-bbs-panel hover:border-bbs-cyan transition-colors group">
                            <span className="font-bold text-[#e0e0e0] group-hover:text-bbs-cyan uppercase mb-1">Windows (x64)</span>
                            <span className="text-xs text-bbs-muted font-mono">resilient-gui-win32-x64.exe</span>
                        </button>
                        <button className="flex flex-col items-start p-4 border border-bbs-panel hover:border-bbs-cyan transition-colors group">
                            <span className="font-bold text-[#e0e0e0] group-hover:text-bbs-cyan uppercase mb-1">Linux (AppImage)</span>
                            <span className="text-xs text-bbs-muted font-mono">resilient-gui-linux-x86_64.AppImage</span>
                        </button>
                    </div>
                </section>

                {/* Mobile Apps */}
                <section className="border border-bbs-panel bg-bbs-surface/30 p-4 rounded-sm relative overflow-hidden">
                    <div className="absolute top-0 right-0 bg-bbs-amber text-black text-[10px] font-bold px-2 py-1 uppercase tracking-widest rounded-bl-sm z-10">Beta</div>
                    <div className="flex items-center space-x-2 mb-4 border-b border-bbs-panel pb-2">
                        <Smartphone className="text-bbs-amber" size={18} />
                        <h3 className="text-lg font-bold text-bbs-amber uppercase tracking-wider">Mobile Mesh Endpoints</h3>
                    </div>

                    <div className="grid grid-cols-2 gap-6">
                        <div className="space-y-3">
                            <h4 className="text-sm font-bold text-[#e0e0e0] uppercase">Android sideloading</h4>
                            <p className="text-xs text-bbs-muted leading-relaxed">
                                Due to App Store restrictions on raw hardware protocol access (LoRa/USB), the full mobile node must be sideloaded.
                            </p>
                            <button className="bbs-button-amber text-sm w-full justify-between items-center flex">
                                <span>Download .APK file</span> <Download size={14} />
                            </button>
                        </div>
                        <div className="space-y-3 flex flex-col h-full pb-2">
                            <h4 className="text-sm font-bold text-[#e0e0e0] uppercase flex items-center space-x-1">
                                <span>iOS / iPadOS</span> <ShieldCheck size={14} className="text-bbs-green ml-1" />
                            </h4>
                            <p className="text-xs text-bbs-muted leading-relaxed flex-1">
                                For iOS, you must use the TestFlight beta link or sideload the IPA using AltStore/Sideloadly.
                            </p>
                            <div className="flex flex-col space-y-2 mt-auto">
                                <a href="#" className="bbs-button text-sm w-full text-center hover:text-white transition-colors flex justify-between items-center group">
                                    <span>Join Apple TestFlight</span> <ChevronRight size={14} className="group-hover:translate-x-1 transition-transform" />
                                </a>
                                <button className="bbs-button text-sm w-full flex justify-between items-center group">
                                    <span>Download .IPA File</span> <Download size={14} className="group-hover:text-bbs-cyan text-bbs-muted transition-colors" />
                                </button>
                            </div>
                        </div>
                    </div>
                </section>

                {/* Headless Daemon */}
                <section className="border border-bbs-panel bg-bbs-surface/30 p-4 rounded-sm">
                    <div className="flex items-center space-x-2 mb-4 border-b border-bbs-panel pb-2">
                        <Terminal className="text-bbs-green" size={18} />
                        <h3 className="text-lg font-bold text-bbs-green uppercase tracking-wider">Headless Daemon / CLI Utilities</h3>
                    </div>

                    <p className="text-sm text-bbs-muted mb-4 leading-relaxed">
                        For headless servers like Raspberry Pi routers, simply pipe the installation shell script from this node directly into bash to compile and register the daemon as a systemd service.
                    </p>

                    <div className="bg-black border border-bbs-panel p-3 rounded-sm font-mono text-bbs-green text-xs flex justify-between items-center relative overflow-hidden group">
                        <div className="absolute inset-0 bg-bbs-green/5 opacity-0 group-hover:opacity-100 transition-opacity" />
                        <span>$ curl -sSL /install.sh | bash</span>
                        <Terminal size={14} className="text-bbs-green opacity-50" />
                    </div>
                </section>

            </div>
        </div>
    );
}
