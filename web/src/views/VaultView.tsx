import { Folder, FolderPlus, FileText, DownloadCloud, Database, ArchiveX, Plus, Link as LinkIcon, ChevronRight, X, Package, ShieldCheck, Share2, Globe, Activity } from 'lucide-react';
import { useState, useEffect } from 'react';
import { CopyableID } from '../components/CopyableID';

type Catalog = { id: string; name: string; description: string; root_hash: string; created_at: number; status?: string; peers?: number };
type Bundle = { id: string; catalog_id: string; parent_bundle_id: string; type?: string; name: string; description: string; created_at: number };
type FileItem = { id: string; catalog_id: string; bundle_id: string; title?: string; path: string; size: number; chunk_hashes: string; source_url?: string };
type Comment = { id: string; author_id: string; content: string; created_at: number };

export function VaultView() {
    const [catalogs, setCatalogs] = useState<Catalog[]>([]);

    // Navigation State
    const [activeCatalog, setActiveCatalog] = useState<Catalog | null>(null);
    const [activeBundleChain, setActiveBundleChain] = useState<Bundle[]>([]); // Breadcrumbs

    // Content State
    const [bundles, setBundles] = useState<Bundle[]>([]);
    const [files, setFiles] = useState<FileItem[]>([]);

    // Chat State
    const [expandedId, setExpandedId] = useState<string | null>(null);
    const [comments, setComments] = useState<Comment[]>([]);
    const [newComment, setNewComment] = useState("");

    // Integrity State
    const [verifyStatuses, setVerifyStatuses] = useState<Record<string, { status: string; corrupted: number; missing: number; valid: number; total: number }>>({});
    const [isVerifying, setIsVerifying] = useState<Record<string, boolean>>({});

    // Modals
    const [showNewCatModal, setShowNewCatModal] = useState(false);
    const [showNewBundleModal, setShowNewBundleModal] = useState(false);
    const [showImportModal, setShowImportModal] = useState(false);
    const [isImporting, setIsImporting] = useState(false);
    const [isSyncing, setIsSyncing] = useState(false);
    const [activeViewerUrl, setActiveViewerUrl] = useState<string | null>(null);
    const [newBundleType, setNewBundleType] = useState<'bundle' | 'folder'>('bundle');

    const [formVals, setFormVals] = useState({ name: '', desc: '', url: '', title: '' });

    // RVX States
    const [showInspectModal, setShowInspectModal] = useState(false);
    const [isInspecting, setIsInspecting] = useState(false);
    const [inspectResult, setInspectResult] = useState<any>(null);
    const [inspectStrategy, setInspectStrategy] = useState<'http' | 'mesh' | 'hybrid'>('hybrid');
    const [isIngesting, setIsIngesting] = useState<{ [key: string]: { active: boolean; percent?: number } }>({});
    const [refreshTrigger, setRefreshTrigger] = useState(0);

    const fetchCatalogs = () => {
        setIsSyncing(true);
        fetch('/api/catalogs')
            .then(res => res.json())
            .then(data => {
                // Mocking Network status metrics for UI flair until integrated
                const enriched = (data || []).map((c: Catalog, i: number) => ({
                    ...c,
                    status: i % 2 === 0 ? 'Seeding' : 'Available',
                    peers: Math.floor(Math.random() * 15)
                }));
                setCatalogs(enriched);
            })
            .catch(err => console.error("Failed to fetch catalogs", err))
            .finally(() => {
            });
    };

    const handleQueueIntake = async (fileId: string, sourceUrl: string, catalogId: string, bundleId: string, title?: string) => {
        if (!sourceUrl) {
            alert("This file has no chunk manifest and no source URL. Click 'Import URL' on the top nav to seed this file manually.");
            return;
        }

        setIsIngesting(prev => ({ ...prev, [fileId]: { active: true } }));
        try {
            const res = await fetch('/api/import/url', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ file_id: fileId, url: sourceUrl, catalog_id: catalogId, bundle_id: bundleId, title: title })
            });
            if (!res.ok) throw new Error("Ingestion request failed");
            // Leave it spinning until the node restarts or chunks are populated
        } catch (err) {
            alert("Ingestion failed: " + err);
            setIsIngesting(prev => ({ ...prev, [fileId]: { active: false } }));
        }
    };

    useEffect(() => {
        fetchCatalogs();
        const interval = setInterval(fetchCatalogs, 3000);
        return () => clearInterval(interval);
    }, []);

    useEffect(() => {
        let previousActiveIds = new Set<string>();

        const fetchIngestions = () => {
            fetch('/api/downloads')
                .then(res => res.json())
                .then(data => {
                    if (data?.transfers) {
                        setIsIngesting(prev => {
                            const next = { ...prev };
                            // @ts-ignore
                            const activeTransfers = Object.values(data.transfers).filter((t: any) => t.status === 'downloading');
                            const activeIds = new Set<string>(activeTransfers.map((t: any) => t.file_id));

                            let justFinished = false;
                            previousActiveIds.forEach(id => {
                                if (!activeIds.has(id)) justFinished = true;
                            });
                            if (justFinished) {
                                setRefreshTrigger(p => p + 1);
                            }
                            previousActiveIds = activeIds;

                            Object.keys(next).forEach(id => {
                                if (!activeIds.has(id)) {
                                    delete next[id];
                                }
                            });

                            activeTransfers.forEach((t: any) => {
                                let percent = 0;
                                if (t.total_chunks > 0) {
                                    percent = Math.round((t.downloaded_chunks / t.total_chunks) * 100);
                                }
                                // @ts-ignore
                                next[t.file_id] = { active: true, percent };
                            });

                            if (JSON.stringify(prev) === JSON.stringify(next)) return prev;
                            return next;
                        });
                    }
                })
                .catch(err => console.error("Failed to fetch active ingestions", err));
        };

        fetchIngestions();
        const interval = setInterval(fetchIngestions, 2000);
        return () => clearInterval(interval);
    }, []);

    // Fetch contents when navigation changes
    useEffect(() => {
        if (!activeCatalog) return;

        let endpoint = `/api/catalogs/${activeCatalog.id}`;
        if (activeBundleChain.length > 0) {
            const currentBundle = activeBundleChain[activeBundleChain.length - 1];
            endpoint = `/api/bundles/${currentBundle.id}`;
        }

        fetch(endpoint)
            .then(res => res.json())
            .then(data => {
                setBundles(data.bundles || []);
                setFiles(data.files || []);
            })
            .catch(err => console.error("Failed to fetch contents", err));

    }, [activeCatalog, activeBundleChain, refreshTrigger]);


    const handleCreateCatalog = () => {
        if (!formVals.name) return;
        fetch('/api/catalogs', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name: formVals.name, description: formVals.desc })
        })
            .then(() => {
                setShowNewCatModal(false);
                setFormVals({ name: '', desc: '', url: '', title: '' });
                fetchCatalogs();
            });
    };

    const handleBootstrapSurvival = () => {
        fetch('/api/bootstrap', { method: 'POST' })
            .then(() => {
                setShowNewCatModal(false);
                fetchCatalogs();
            })
            .catch(err => alert("Failed to trigger seed: " + err));
    };

    const handleCreateBundle = () => {
        if (!formVals.name || !activeCatalog) return;

        const parentID = activeBundleChain.length > 0 ? activeBundleChain[activeBundleChain.length - 1].id : "";

        fetch('/api/bundles', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                catalog_id: activeCatalog.id,
                parent_bundle_id: parentID,
                type: newBundleType,
                name: formVals.name,
                description: formVals.desc
            })
        })
            .then(() => {
                setShowNewBundleModal(false);
                setFormVals({ name: '', desc: '', url: '', title: '' });
                // Trigger re-render of contents
                setActiveBundleChain([...activeBundleChain]);
            });
    };

    const handleImportURL = () => {
        if (!formVals.url || !activeCatalog) return;

        const parentID = activeBundleChain.length > 0 ? activeBundleChain[activeBundleChain.length - 1].id : "";

        setIsImporting(true);
        fetch('/api/import/url', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                catalog_id: activeCatalog.id,
                bundle_id: parentID,
                url: formVals.url,
                title: formVals.title
            })
        })
            .then(() => {
                setShowImportModal(false);
                setFormVals({ name: '', desc: '', url: '', title: '' });
                alert("Background ingestion started! The file will appear once chunking begins.");
            })
            .catch(err => alert("Ingestion failed: " + err))
            .finally(() => setIsImporting(false));
    };

    const handleExport = (e: React.MouseEvent, id: string, type: 'catalog' | 'bundle' | 'file') => {
        e.stopPropagation();
        window.open(`/api/export?id=${id}&type=${type}`, '_blank');
    };

    const handleInspectRVX = () => {
        if (!formVals.url) return;
        setIsInspecting(true);
        fetch('/api/inspect', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ url: formVals.url })
        })
            .then(async res => {
                if (!res.ok) {
                    const text = await res.text();
                    throw new Error(text);
                }
                return res.json();
            })
            .then(data => {
                setInspectResult(data);
                setShowInspectModal(true);
                setShowImportModal(false);
            })
            .catch(err => alert("Advanced RVX Inspection Failed. Make sure it's a valid remote RVX file.\n" + err))
            .finally(() => setIsInspecting(false));
    };

    const handleExecuteRVX = () => {
        if (!inspectResult || !formVals.url) return;
        setIsImporting(true);
        fetch('/api/import/rvx', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                url: formVals.url,
                header: inspectResult.header,
                strategy: inspectStrategy
            })
        })
            .then(() => {
                setShowInspectModal(false);
                setInspectResult(null);
                setFormVals({ name: '', desc: '', url: '', title: '' });
                alert("RVX Ingestion started securely in the background using strategy: " + inspectStrategy.toUpperCase());
                if (activeCatalog) {
                    setActiveBundleChain([...activeBundleChain]);
                } else {
                    fetchCatalogs();
                }
            })
            .catch(err => alert("Ingestion failed: " + err))
            .finally(() => setIsImporting(false));
    };


    useEffect(() => {
        if (expandedId) {
            fetch(`/api/files/${expandedId}/comments`)
                .then(res => res.json())
                .then(data => setComments(data || []))
                .catch(err => console.error("Failed to fetch comments", err));
        }
    }, [expandedId]);

    const handlePostComment = () => {
        if (!newComment.trim() || !expandedId) return;

        fetch('/api/chat', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ content: newComment, ref_target_id: expandedId }),
        })
            .then(res => res.json())
            .then(msg => {
                setComments([...comments, msg]);
                setNewComment("");
            })
            .catch(err => console.error("Failed to post comment", err));
    };

    const handleVerify = (fileId: string) => {
        setIsVerifying(prev => ({ ...prev, [fileId]: true }));
        fetch('/api/verify', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ file_id: fileId })
        })
            .then(res => res.json())
            .then(data => {
                setVerifyStatuses(prev => ({ ...prev, [fileId]: data }));
            })
            .catch(err => console.error("Verification failed", err))
            .finally(() => {
                setIsVerifying(prev => ({ ...prev, [fileId]: false }));
            });
    };

    const handleFetchBundle = (bundleId: string) => {
        // Fetch all files for this bundle and initiate download requests for each piece
        fetch(`/api/bundles/${bundleId}`)
            .then(res => res.json())
            .then(data => {
                const innerFiles = data.files || [];
                innerFiles.forEach((f: FileItem) => {
                    fetch('/api/fetch', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ file_id: f.id })
                    }).catch(err => console.error("Failed to fetch folder item " + f.id, err));
                });

                // Recursion for nested bundles would go here if needed, but keeping it flat 1-layer for now
            })
            .catch(err => console.error("Failed to enumerate bundle", err));
    };

    return (
        <div className="flex flex-col h-full animate-in fade-in duration-300 relative">
            <header className="mb-6 flex justify-between items-end border-b border-bbs-panel pb-2">
                <div>
                    <h2 className="text-2xl font-bold uppercase tracking-widest text-bbs-cyan drop-shadow-[0_0_8px_var(--color-bbs-cyan-dim)]">Vault Explorer</h2>
                    <p className="text-bbs-muted text-sm mt-1">Local Addressable Storage & Remote Catalogs</p>
                </div>
                <div className="flex space-x-3">
                    <button onClick={() => setShowNewCatModal(true)} className="bbs-button text-xs flex items-center space-x-1">
                        <Plus size={14} />
                        <span>New Catalog</span>
                    </button>
                    <button
                        onClick={fetchCatalogs}
                        disabled={isSyncing}
                        className={`text-xs flex items-center space-x-2 transition-all ${isSyncing ? 'bbs-button opacity-80 cursor-wait' : 'bbs-button-cyan'}`}
                    >
                        <Database size={14} className={isSyncing ? "animate-spin" : ""} />
                        <span>{isSyncing ? "SYNCING..." : "SYNC DHT"}</span>
                    </button>
                </div>
            </header>

            <div className="grid grid-cols-4 gap-4 flex-1 overflow-hidden">
                {/* Left pane: Root Catalogs List */}
                <div className="col-span-1 border-r border-bbs-panel pr-4 flex flex-col overflow-y-auto">
                    <div className="text-xs uppercase text-bbs-cyan font-bold tracking-widest mb-3 border-b border-bbs-cyan/30 pb-1">Root Catalogs</div>
                    {catalogs.map((cat) => (
                        <button
                            key={cat.id}
                            onClick={() => { setActiveCatalog(cat); setActiveBundleChain([]); }}
                            className={`w-full text-left px-2 py-2 mb-1 flex items-center space-x-2 text-sm transition-colors ${activeCatalog?.id === cat.id ? 'text-bbs-cyan bg-bbs-cyan/10 border-l-2 border-bbs-cyan' : 'text-bbs-muted hover:text-bbs-text hover:bg-bbs-panel/30 border-l-2 border-transparent'}`}
                        >
                            <Database size={14} className={activeCatalog?.id === cat.id ? "text-bbs-cyan" : "text-bbs-muted"} />
                            <span className="truncate">{cat.name}</span>
                        </button>
                    ))}
                    {catalogs.length === 0 && (
                        <div className="text-xs text-bbs-muted italic p-2 opacity-50">No catalogs constructed yet.</div>
                    )}
                </div>

                {/* Right pane: Inner Contents Explorer */}
                <div className="col-span-3 pl-2 flex flex-col space-y-3 overflow-hidden">
                    {!activeCatalog ? (
                        <div className="h-full flex items-center justify-center opacity-30 text-bbs-muted flex-col">
                            <Database size={48} className="mb-4" />
                            <p>Select a Catalog to begin exploration.</p>
                        </div>
                    ) : (
                        <>
                            {/* Explorer Action Bar & Breadcrumbs */}
                            <div className="flex justify-between items-center bg-bbs-panel/20 p-2 border border-bbs-panel rounded">
                                <div className="flex items-center space-x-2 text-sm text-bbs-text custom-scrollbar overflow-x-auto whitespace-nowrap">
                                    <button onClick={() => setActiveBundleChain([])} className="hover:text-bbs-cyan transition-colors">{activeCatalog.name}</button>
                                    {activeBundleChain.map((b, idx) => (
                                        <div key={b.id} className="flex items-center space-x-2">
                                            <ChevronRight size={14} className="text-bbs-muted" />
                                            <button
                                                onClick={() => setActiveBundleChain(activeBundleChain.slice(0, idx + 1))}
                                                className="hover:text-bbs-cyan transition-colors"
                                            >
                                                {b.name}
                                            </button>
                                        </div>
                                    ))}
                                </div>
                                <div className="flex space-x-2 shrink-0">
                                    <button onClick={() => { setNewBundleType('folder'); setShowNewBundleModal(true); }} className="bbs-button text-xs py-1 px-2 flex items-center space-x-1">
                                        <FolderPlus size={12} />
                                        <span>+ Folder</span>
                                    </button>
                                    <button onClick={() => { setNewBundleType('bundle'); setShowNewBundleModal(true); }} className="bbs-button text-xs py-1 px-2 flex items-center space-x-1">
                                        <Package size={12} />
                                        <span>+ Bundle</span>
                                    </button>
                                    <button onClick={(e) => handleExport(e, activeBundleChain.length > 0 ? activeBundleChain[activeBundleChain.length - 1].id : activeCatalog.id, activeBundleChain.length > 0 ? 'bundle' : 'catalog')} className="bbs-button text-xs py-1 px-2 flex items-center space-x-1" title="Export this collection to an .rvx file">
                                        <Share2 size={12} />
                                        <span>Export .rvx</span>
                                    </button>
                                    <button onClick={() => setShowImportModal(true)} className="bbs-button-cyan text-xs py-1 px-2 flex items-center space-x-1">
                                        <LinkIcon size={12} />
                                        <span>Import URL</span>
                                    </button>
                                </div>
                            </div>

                            {/* Files & Bundles List */}
                            <div className="flex-1 overflow-y-auto custom-scrollbar flex flex-col">
                                {bundles.length === 0 && files.length === 0 && (
                                    <div className="text-center italic text-bbs-muted mt-10">This volume is entirely void of data.</div>
                                )}

                                {bundles.map(b => (
                                    <button
                                        key={b.id}
                                        onClick={() => setActiveBundleChain([...activeBundleChain, b])}
                                        className="group flex flex-col text-left mb-2 p-3 border border-bbs-panel bg-bbs-surface hover:border-bbs-amber transition-all duration-200"
                                    >
                                        <div className="flex items-center justify-between">
                                            <div className="flex items-center space-x-3">
                                                {b.type === 'folder' ? <Folder size={18} className="text-bbs-amber" /> : <Package size={18} className="text-bbs-amber" />}
                                                <span className="font-bold text-bbs-text transition-colors group-hover:text-bbs-amber" title={b.id}>{b.name}</span>
                                            </div>
                                            <div className="flex space-x-4 items-center">
                                                <button
                                                    onClick={(e) => handleExport(e, b.id, 'bundle')}
                                                    className="bbs-button py-0.5 px-2 opacity-0 group-hover:opacity-100 transition-opacity flex items-center space-x-1 hover:text-bbs-cyan hover:border-bbs-cyan"
                                                    title="Export folder to .rvx"
                                                >
                                                    <Share2 size={12} /> <span className="text-[10px]">EXPORT</span>
                                                </button>
                                                <button
                                                    onClick={(e) => { e.stopPropagation(); handleFetchBundle(b.id); }}
                                                    className="bbs-button-cyan py-0.5 px-2 opacity-0 group-hover:opacity-100 transition-opacity flex items-center space-x-1"
                                                    title="Sync entire folder contents from mesh network"
                                                >
                                                    <DownloadCloud size={12} /> <span className="text-[10px]">SYNC ALL</span>
                                                </button>
                                                <span className="text-xs text-bbs-muted">{new Date(b.created_at * 1000).toLocaleDateString()}</span>
                                            </div>
                                        </div>
                                        <p className="text-xs text-bbs-muted mt-1 ml-7 italic opacity-80 truncate">{b.description || (b.type === 'folder' ? 'Local Directory' : 'Nested Bundle')}</p>
                                    </button>
                                ))}

                                {files.map(f => (
                                    <div key={f.id} className="flex flex-col">
                                        <div className="group flex items-center justify-between p-3 border border-bbs-panel bg-bbs-surface hover:border-bbs-cyan transition-all duration-200">
                                            <div className="w-[60%] flex items-center space-x-3 min-w-0">
                                                {f.path.includes('.zim') || f.path.includes('.tar') || f.path.includes('.zip') ?
                                                    <ArchiveX size={18} className="text-bbs-cyan shrink-0" /> :
                                                    <FileText size={18} className="text-bbs-muted shrink-0" />
                                                }
                                                <div className="flex flex-col min-w-0 flex-1 py-1 pr-4">
                                                    <span className="font-bold text-bbs-text whitespace-normal break-words leading-tight" title={f.title || f.path}>{f.title || f.path}</span>
                                                    {f.title && f.title !== f.path && (
                                                        <span className="text-[10px] font-mono text-bbs-muted mt-0.5">{f.path}</span>
                                                    )}
                                                    <div className="mt-1 flex items-center space-x-2">
                                                        {f.chunk_hashes !== "[]" && (
                                                            <CopyableID label="CID" value={JSON.parse(f.chunk_hashes)[0] || 'Unknown'} />
                                                        )}

                                                        {f.chunk_hashes === "[]" ? (
                                                            <div className="text-[10px] px-2 py-0.5 rounded border border-bbs-amber text-bbs-amber bg-bbs-amber/10 tracking-widest font-semibold flex items-center space-x-1">
                                                                <span className="w-1.5 h-1.5 rounded-full bg-bbs-amber animate-pulse"></span>
                                                                <span>AWAITING INGESTION</span>
                                                            </div>
                                                        ) : verifyStatuses[f.id] ? (
                                                            <div className={`text-[10px] px-2 py-0.5 rounded border ${verifyStatuses[f.id].status === 'valid' ? 'border-bbs-teal text-bbs-teal bg-bbs-teal/10' : 'border-bbs-red text-bbs-red bg-bbs-red/10'}`}>
                                                                {verifyStatuses[f.id].status === 'valid' ?
                                                                    `VALID (${verifyStatuses[f.id].valid}/${verifyStatuses[f.id].total} BLOCKS)` :
                                                                    `CORRUPT (Valid: ${verifyStatuses[f.id].valid}, Missing: ${verifyStatuses[f.id].missing}, Corrupted: ${verifyStatuses[f.id].corrupted})`}
                                                            </div>
                                                        ) : (
                                                            <button
                                                                onClick={() => handleVerify(f.id)}
                                                                disabled={isVerifying[f.id]}
                                                                className={`text-[10px] px-2 py-0.5 rounded border border-bbs-panel text-bbs-muted hover:text-bbs-cyan hover:border-bbs-cyan transition-colors flex items-center space-x-1 ${isVerifying[f.id] ? 'opacity-50 cursor-wait' : ''}`}
                                                                title="Verify cryptographic hash integrity of local chunks against CID manifest"
                                                            >
                                                                {isVerifying[f.id] ? <span className="animate-spin">⟳</span> : <ShieldCheck size={10} />}
                                                                <span>{isVerifying[f.id] ? 'VERIFYING...' : 'VERIFY INTEGRITY'}</span>
                                                            </button>
                                                        )}
                                                    </div>
                                                </div>
                                            </div>

                                            <div className="w-1/6 text-right text-xs font-mono text-bbs-muted">
                                                {(f.size / 1024 / 1024).toFixed(2)} MB
                                            </div>

                                            <div className="flex space-x-2 items-center">
                                                {f.chunk_hashes === "[]" ? (
                                                    <button
                                                        onClick={() => handleQueueIntake(f.id, f.source_url || '', f.catalog_id || '', f.bundle_id || '', f.title)}
                                                        disabled={isIngesting[f.id]?.active}
                                                        className={`bbs-button-amber py-1 px-3 opacity-0 group-hover:opacity-100 transition-opacity whitespace-nowrap text-xs flex items-center space-x-1 ${isIngesting[f.id]?.active ? 'opacity-100 cursor-wait' : ''}`}
                                                    >
                                                        {isIngesting[f.id]?.active ? (
                                                            <>
                                                                <span className="animate-spin w-3 h-3 border-2 border-bbs-amber border-t-transparent rounded-full" />
                                                                <span>{isIngesting[f.id].percent !== undefined && isIngesting[f.id].percent! > 0 ? `INGESTING... ${isIngesting[f.id].percent}%` : 'INGESTING...'}</span>
                                                            </>
                                                        ) : (
                                                            <>
                                                                <DownloadCloud size={14} /> <span>QUEUE INTAKE</span>
                                                            </>
                                                        )}
                                                    </button>
                                                ) : (
                                                    <>
                                                        <button
                                                            onClick={(e) => handleExport(e, f.id, 'file')}
                                                            className="bbs-button py-1 px-2 opacity-0 group-hover:opacity-100 transition-opacity hover:text-bbs-cyan hover:border-bbs-cyan"
                                                            title="Export file to .rvx"
                                                        >
                                                            <Share2 size={14} />
                                                        </button>
                                                        {f.path.match(/\.(zip|tar|zim|warc|wacz)$/i) ? (
                                                            <button
                                                                onClick={() => setActiveViewerUrl(`/api/archives/${f.id}/${f.path.match(/\.zim$/i) ? '' : 'index.html'}`)}
                                                                className="bbs-button py-1 px-2 opacity-0 group-hover:opacity-100 transition-opacity whitespace-nowrap"
                                                            >
                                                                Mount
                                                            </button>
                                                        ) : (
                                                            <button
                                                                onClick={() => setExpandedId(expandedId === f.id ? null : f.id)}
                                                                className="bbs-button py-1 px-2 opacity-0 group-hover:opacity-100 transition-opacity"
                                                            >
                                                                Discuss
                                                            </button>
                                                        )}
                                                        <button
                                                            onClick={() => fetch('/api/fetch', {
                                                                method: 'POST',
                                                                headers: { 'Content-Type': 'application/json' },
                                                                body: JSON.stringify({ file_id: f.id })
                                                            }).catch(err => console.error(err))}
                                                            className="bbs-button-cyan py-1 px-2 opacity-0 group-hover:opacity-100 transition-opacity"
                                                        >
                                                            <DownloadCloud size={14} />
                                                        </button>
                                                    </>
                                                )}
                                            </div>
                                        </div>

                                        {expandedId === f.id && (
                                            <div className="ml-8 border-l border-bbs-panel pl-4 py-2 mt-[-1px] mb-2 space-y-3 relative bg-bbs-bg shadow-inner">
                                                <div className="text-[10px] uppercase text-bbs-cyan tracking-widest font-bold">Forum thread for / {f.path}</div>

                                                {comments.length === 0 ? (
                                                    <div className="text-xs text-bbs-muted italic">No comments yet. Start the discussion.</div>
                                                ) : (
                                                    <div className="space-y-2 max-h-48 overflow-y-auto custom-scrollbar pr-2">
                                                        {comments.map(c => (
                                                            <div key={c.id} className="text-sm bg-bbs-surface p-2 border border-bbs-panel text-bbs-text shadow-sm">
                                                                <div className="text-[10px] text-bbs-muted flex justify-between mb-1">
                                                                    <span className="text-bbs-amber" title={c.author_id}>ID: {c.author_id.substring(0, 8)}...</span>
                                                                    <span>{new Date(c.created_at * 1000).toLocaleString()}</span>
                                                                </div>
                                                                <div>{c.content}</div>
                                                            </div>
                                                        ))}
                                                    </div>
                                                )}

                                                <div className="flex space-x-2">
                                                    <input
                                                        type="text"
                                                        className="bbs-input-cyan flex-1 text-sm py-1"
                                                        placeholder="Transmit a comment to the mesh..."
                                                        value={newComment}
                                                        onChange={e => setNewComment(e.target.value)}
                                                        onKeyDown={e => e.key === 'Enter' && handlePostComment()}
                                                    />
                                                    <button onClick={handlePostComment} className="bbs-button-cyan text-xs font-bold uppercase px-3">
                                                        TX
                                                    </button>
                                                </div>
                                            </div>
                                        )}
                                    </div>
                                ))}
                            </div>
                        </>
                    )}
                </div>
            </div>

            {/* -- MODALS -- */}

            {/* New Catalog Modal */}
            {
                showNewCatModal && (
                    <div className="fixed inset-0 bg-black/80 flex items-center justify-center z-50">
                        <div className="bg-bbs-bg border border-bbs-cyan p-6 shadow-[0_0_15px_var(--color-bbs-cyan-dim)] w-[400px]">
                            <h3 className="text-bbs-cyan text-lg font-bold uppercase mb-4 tracking-widest border-b border-bbs-cyan/30 pb-2">Initialize Catalog</h3>
                            <div className="space-y-4">
                                <div>
                                    <label className="block text-xs uppercase text-bbs-muted mb-1">Catalog Name</label>
                                    <input value={formVals.name} onChange={e => setFormVals({ ...formVals, name: e.target.value })} className="w-full bbs-input text-sm" placeholder="e.g. Wiki Archive 2024" />
                                </div>
                                <div>
                                    <label className="block text-xs uppercase text-bbs-muted mb-1">Description</label>
                                    <textarea value={formVals.desc} onChange={e => setFormVals({ ...formVals, desc: e.target.value })} className="w-full bbs-input text-sm h-20" placeholder="Purpose of this root partition..." />
                                </div>
                                <div className="flex justify-between items-center pt-4 border-t border-bbs-panel mt-2">
                                    <button onClick={handleBootstrapSurvival} className="bbs-button text-xs text-bbs-amber border-bbs-amber/50 hover:bg-bbs-amber/10 flex items-center space-x-1">
                                        <ShieldCheck size={12} />
                                        <span>Initialize Survival Seed</span>
                                    </button>
                                    <div className="flex space-x-3">
                                        <button onClick={() => setShowNewCatModal(false)} className="bbs-button text-sm">Cancel</button>
                                        <button onClick={handleCreateCatalog} className="bbs-button-cyan text-sm">Construct</button>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                )
            }

            {/* New Bundle Modal */}
            {
                showNewBundleModal && (
                    <div className="fixed inset-0 bg-black/80 flex items-center justify-center z-50">
                        <div className="bg-bbs-bg border border-bbs-amber p-6 shadow-[0_0_15px_var(--color-bbs-amber-dim)] w-[400px]">
                            <h3 className="text-bbs-amber text-lg font-bold uppercase mb-4 tracking-widest border-b border-bbs-amber/30 pb-2">Create {newBundleType === 'folder' ? 'Folder' : 'Immuatable Bundle'}</h3>
                            <div className="space-y-4">
                                <div>
                                    <label className="block text-xs uppercase text-bbs-muted mb-1">Target</label>
                                    <div className="text-xs bg-bbs-panel/30 p-2 font-mono text-bbs-cyan">/{activeCatalog?.name}/{activeBundleChain.map(b => b.name).join('/')}</div>
                                </div>
                                <div>
                                    <label className="block text-xs uppercase text-bbs-muted mb-1">{newBundleType === 'folder' ? 'Folder Name' : 'Bundle Target'}</label>
                                    <input value={formVals.name} onChange={e => setFormVals({ ...formVals, name: e.target.value })} className="w-full bbs-input-amber text-sm" placeholder={newBundleType === 'folder' ? 'e.g. Schematics' : 'e.g. Medical PDFs'} />
                                </div>
                                <div>
                                    <label className="block text-xs uppercase text-bbs-muted mb-1">Description</label>
                                    <textarea value={formVals.desc} onChange={e => setFormVals({ ...formVals, desc: e.target.value })} className="w-full bbs-input-amber text-sm h-16" placeholder="Optional context..." />
                                </div>
                                <div className="flex justify-end space-x-3 pt-2">
                                    <button onClick={() => setShowNewBundleModal(false)} className="bbs-button text-sm">Cancel</button>
                                    <button onClick={handleCreateBundle} className="bbs-button-amber text-sm">Mkdir</button>
                                </div>
                            </div>
                        </div>
                    </div>
                )
            }

            {/* Import URL Modal */}
            {
                showImportModal && (
                    <div className="fixed inset-0 bg-black/80 flex items-center justify-center z-50">
                        <div className="bg-bbs-bg border border-bbs-cyan p-6 shadow-[0_0_20px_var(--color-bbs-cyan-dim)] w-[500px]">
                            <div className="flex items-center space-x-2 mb-4 border-b border-bbs-cyan/30 pb-2">
                                <DownloadCloud className="text-bbs-cyan" />
                                <h3 className="text-bbs-cyan text-lg font-bold uppercase tracking-widest">Ingest From URL</h3>
                            </div>
                            <div className="space-y-4">
                                <div>
                                    <label className="block text-xs uppercase text-bbs-muted mb-1">Target Destination</label>
                                    <div className="text-xs bg-bbs-panel/30 p-2 font-mono mb-2">/{activeCatalog?.name}/{activeBundleChain.map(b => b.name).join('/')}</div>
                                    <p className="text-xs text-bbs-muted italic">The daemon will stream this URL into CAS distributed chunks.</p>
                                </div>
                                <div>
                                    <label className="block text-xs uppercase text-bbs-teal mb-1">Title (Optional Human-Readable)</label>
                                    <input value={formVals.title} onChange={e => setFormVals({ ...formVals, title: e.target.value })} className="w-full bbs-input-cyan text-sm mb-3" placeholder="e.g. Global Wikipedia 2024" />

                                    <label className="block text-xs uppercase text-bbs-teal mb-1">Direct URL Payload</label>
                                    <input value={formVals.url} onChange={e => setFormVals({ ...formVals, url: e.target.value })} className="w-full bbs-input-cyan text-sm" placeholder="https://example.com/massive_file.zip" />
                                </div>
                                <div className="flex justify-end space-x-3 pt-2">
                                    <button onClick={() => setShowImportModal(false)} disabled={isImporting || isInspecting} className="bbs-button text-sm disabled:opacity-50">Cancel</button>
                                    <button onClick={() => formVals.url.endsWith('.rvx') ? handleInspectRVX() : handleImportURL()} disabled={isImporting || isInspecting} className="bbs-button-cyan text-sm flex items-center space-x-1 disabled:opacity-50 disabled:cursor-not-allowed">
                                        {(isImporting || isInspecting) ? <Activity size={14} className="animate-spin" /> : <DownloadCloud size={14} />}
                                        <span>{formVals.url.endsWith('.rvx') ? (isInspecting ? 'INSPECTING...' : 'INSPECT .RVX') : (isImporting ? "INITIALIZING..." : "COMMENCE STREAM")}</span>
                                    </button>
                                </div>
                            </div>
                        </div>
                    </div>
                )
            }

            {/* RVX Inspect Modal */}
            {
                showInspectModal && inspectResult && (
                    <div className="fixed inset-0 bg-black/80 flex items-center justify-center z-50">
                        <div className="bg-bbs-bg border border-bbs-cyan p-6 shadow-[0_0_20px_var(--color-bbs-cyan-dim)] w-[600px] flex flex-col max-h-[90vh]">
                            <div className="flex items-center space-x-2 mb-4 border-b border-bbs-cyan/30 pb-2">
                                <Share2 className="text-bbs-cyan" />
                                <h3 className="text-bbs-cyan text-lg font-bold uppercase tracking-widest">RVX Import Preview</h3>
                            </div>

                            <div className="flex-1 overflow-y-auto custom-scrollbar pr-2 space-y-4">
                                <div className="bg-bbs-surface border border-bbs-panel p-3">
                                    <h4 className="text-xs uppercase text-bbs-muted font-bold tracking-widest mb-2 border-b border-bbs-panel pb-1 flex justify-between">
                                        <span>Metadata</span>
                                        <span className="text-bbs-cyan">.{inspectResult.header.type}</span>
                                    </h4>
                                    <p className="font-bold text-sm text-bbs-text">{inspectResult.header.metadata.title || inspectResult.header.metadata.name}</p>
                                    <p className="text-xs text-bbs-muted mt-1 italic">{inspectResult.header.metadata.description || 'No description provided.'}</p>
                                    <div className="text-[10px] uppercase font-mono mt-2 pt-2 border-t border-bbs-panel opacity-70">
                                        Data Hash: {inspectResult.header.data_hash.substring(0, 32)}...
                                    </div>
                                </div>

                                <div className="bg-bbs-surface border border-bbs-panel p-3">
                                    <h4 className="text-xs uppercase text-bbs-amber font-bold tracking-widest mb-2 border-b border-bbs-panel pb-1">Mesh Availability</h4>
                                    <div className="flex items-center justify-between mb-1">
                                        <span className="text-xs text-bbs-muted">Total Blocks Required:</span>
                                        <span className="text-sm font-mono text-bbs-text">{inspectResult.mesh_availability.total_chunks}</span>
                                    </div>
                                    <div className="flex items-center justify-between mb-1">
                                        <span className="text-xs text-bbs-muted">Blocks Stored Locally:</span>
                                        <span className="text-sm font-mono text-bbs-teal">{inspectResult.mesh_availability.local_chunks}</span>
                                    </div>
                                    <div className="flex items-center justify-between mb-1">
                                        <span className="text-xs text-bbs-muted">Blocks Available On Mesh:</span>
                                        <span className="text-sm font-mono text-bbs-cyan">{inspectResult.mesh_availability.peer_chunks}</span>
                                    </div>
                                </div>

                                <div className="space-y-2">
                                    <h4 className="text-xs uppercase text-bbs-muted font-bold tracking-widest border-b border-bbs-panel pb-1">Ingestion Strategy</h4>
                                    <div className="grid grid-cols-3 gap-2">
                                        <button
                                            onClick={() => setInspectStrategy('http')}
                                            className={`p-2 border text-left flex flex-col items-center justify-center transition-colors ${inspectStrategy === 'http' ? 'border-bbs-cyan bg-bbs-cyan/10 text-bbs-cyan' : 'border-bbs-panel bg-bbs-surface text-bbs-muted hover:border-bbs-cyan/50'}`}
                                        >
                                            <Globe size={18} className="mb-1" />
                                            <span className="text-xs font-bold uppercase">HTTP Only</span>
                                            <span className="text-[9px] text-center mt-1">Direct from source server</span>
                                        </button>
                                        <button
                                            onClick={() => setInspectStrategy('mesh')}
                                            className={`p-2 border text-left flex flex-col items-center justify-center transition-colors ${inspectStrategy === 'mesh' ? 'border-bbs-amber bg-bbs-amber/10 text-bbs-amber' : 'border-bbs-panel bg-bbs-surface text-bbs-muted hover:border-bbs-cyan/50'}`}
                                        >
                                            <Database size={18} className="mb-1" />
                                            <span className="text-xs font-bold uppercase">Mesh Only</span>
                                            <span className="text-[9px] text-center mt-1">P2P Kademlia Swarm</span>
                                        </button>
                                        <button
                                            onClick={() => setInspectStrategy('hybrid')}
                                            className={`p-2 border text-left flex flex-col items-center justify-center transition-colors ${inspectStrategy === 'hybrid' ? 'border-bbs-teal bg-bbs-teal/10 text-bbs-teal' : 'border-bbs-panel bg-bbs-surface text-bbs-muted hover:border-bbs-cyan/50'}`}
                                        >
                                            <Activity size={18} className="mb-1" />
                                            <span className="text-xs font-bold uppercase">Hybrid Boost</span>
                                            <span className="text-[9px] text-center mt-1">Parallel Swarm + HTTP</span>
                                        </button>
                                    </div>
                                </div>
                            </div>

                            <div className="flex justify-end space-x-3 pt-4 border-t border-bbs-cyan/30 mt-4">
                                <button onClick={() => { setShowInspectModal(false); setInspectResult(null); }} className="bbs-button text-sm">Cancel</button>
                                <button onClick={handleExecuteRVX} disabled={isImporting} className="bbs-button-cyan text-sm flex items-center space-x-1 disabled:opacity-50 disabled:cursor-wait">
                                    <DownloadCloud size={14} className={isImporting ? 'animate-pulse' : ''} />
                                    <span>{isImporting ? 'PROVISIONING...' : 'EXECUTE INGESTION'}</span>
                                </button>
                            </div>
                        </div>
                    </div>
                )
            }

            {/* Inline Document Viewer Modal */}
            {activeViewerUrl && (
                <div className="fixed inset-0 bg-black/95 z-[100] flex flex-col">
                    <div className="flex justify-between items-center p-2 bg-bbs-surface border-b border-bbs-panel shrink-0">
                        <div className="flex flex-col ml-2">
                            <span className="text-bbs-cyan text-sm font-bold tracking-widest uppercase">Archive Viewer</span>
                            <span className="text-[10px] text-bbs-muted font-mono">{activeViewerUrl}</span>
                        </div>
                        <button
                            onClick={() => setActiveViewerUrl(null)}
                            className="bbs-button text-xs flex items-center space-x-1 hover:text-bbs-red hover:border-bbs-red"
                        >
                            <X size={14} /> <span>UNMOUNT</span>
                        </button>
                    </div>
                    <iframe
                        src={activeViewerUrl}
                        className="flex-1 w-full bg-white"
                        title="Document Viewer"
                        sandbox="allow-scripts allow-same-origin allow-popups allow-forms"
                    />
                </div>
            )}
        </div >
    );
}
