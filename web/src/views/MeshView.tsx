import { Map as MapIcon, Wifi, Radio, Lock, Copy, CheckCircle2 } from 'lucide-react';
import { useState, useEffect, useRef } from 'react';
import ForceGraph2D from 'react-force-graph-2d';

type Peer = {
    id: string;
    name?: string;
    status: string;
    latency: string;
    route: string;
    trust: string;
};

export function MeshView() {
    const [peers, setPeers] = useState<Peer[]>([]);
    const [dimensions, setDimensions] = useState({ width: 0, height: 0 });
    const [copiedId, setCopiedId] = useState<string | null>(null);
    const [hoverNode, setHoverNode] = useState<any>(null);
    const [selectedNode, setSelectedNode] = useState<string | null>(null);
    const [info, setInfo] = useState<{ node_id: string, node_name?: string } | null>(null);
    const [config, setConfig] = useState<any>(null);

    const fgRef = useRef<any>(null);
    const lastInteractionTime = useRef<number>(0);
    const lastPeerCount = useRef<number>(0);
    const containerRef = useRef<HTMLDivElement>(null);
    const listRef = useRef<HTMLDivElement>(null);
    const itemRefs = useRef<{ [key: string]: HTMLDivElement | null }>({});

    const handleCopy = (id: string) => {
        navigator.clipboard.writeText(id);
        setCopiedId(id);
        setTimeout(() => setCopiedId(null), 2000);
    };

    useEffect(() => {
        const fetchPeers = () => {
            fetch('/api/peers')
                .then(res => res.json())
                .then(data => setPeers(data || []))
                .catch(err => console.error("Failed to fetch peers", err));
        };
        fetchPeers();
        const interval = setInterval(fetchPeers, 2500);

        fetch('/api/info')
            .then(res => res.json())
            .then(data => setInfo(data))
            .catch(err => console.error("Failed to fetch info", err));

        fetch('/api/config')
            .then(res => res.json())
            .then(data => setConfig(data))
            .catch(err => console.error("Failed to fetch config", err));

        return () => clearInterval(interval);
    }, []);

    useEffect(() => {
        if (containerRef.current) {
            setDimensions({
                width: containerRef.current.offsetWidth,
                height: containerRef.current.offsetHeight
            });
        }
        const handleResize = () => {
            if (containerRef.current) {
                setDimensions({
                    width: containerRef.current.offsetWidth,
                    height: containerRef.current.offsetHeight
                });
            }
        };
        window.addEventListener('resize', handleResize);
        return () => window.removeEventListener('resize', handleResize);
    }, []);

    // Apply custom physics to spread nodes out and prevent label overlap
    useEffect(() => {
        if (fgRef.current) {
            fgRef.current.d3Force('charge').strength(-400); // Default is -30. High negative means intense repulsion.
            fgRef.current.d3Force('link').distance(70); // Default is 30. Increase tether distance.
        }
    }, [peers.length]);

    // Dynamic Auto-Zoom on Topology Change and Physics Drift
    useEffect(() => {
        // If a new node joins the network, FORCE the interaction timer forward.
        // D3 physics needs 5-10 seconds to physically push the new node outward to expand the bounding box.
        // If we zoomToFit immediately, we zoom into a microscopic clump before it expands.
        if (peers.length > lastPeerCount.current) {
            lastInteractionTime.current = Date.now();
        }
        lastPeerCount.current = peers.length;

        const autoZoomInterval = setInterval(() => {
            if (fgRef.current && dimensions.width > 0 && peers.length > 0) {
                // Buffer the delay slightly higher to ensure animations have absolutely settled
                const delayMs = ((config?.auto_zoom_delay_secs ?? 60) + 2) * 1000;
                if (Date.now() - lastInteractionTime.current > delayMs) {
                    fgRef.current.zoomToFit(2000, Math.min(dimensions.width, dimensions.height) * 0.15);
                }
            }
        }, 3000);

        return () => clearInterval(autoZoomInterval);
    }, [peers.length, dimensions, config]);

    const prevNodesMap = useRef<Map<string, any>>(new Map());
    const prevLinksMap = useRef<Map<string, any>>(new Map());

    // Maintain stable object and array references for ForceGraph bounds detection
    const nodesRef = useRef<any[]>([]);
    const linksRef = useRef<any[]>([]);
    const [graphDataProp, setGraphDataProp] = useState({ nodes: [] as any[], links: [] as any[] });

    useEffect(() => {
        let structuralChange = false;

        let selfNode = prevNodesMap.current.get("self");
        if (!selfNode) {
            selfNode = { id: "self", group: "self", currentRadius: 0 };
            prevNodesMap.current.set("self", selfNode);
            nodesRef.current.push(selfNode);
            structuralChange = true;
        }

        // Track active peer IDs to prune dead nodes
        const activeIds = new Set<string>(["self"]);

        // Add or update nodes (Mutable)
        peers.forEach(p => {
            activeIds.add(p.id);
            let existingNode = prevNodesMap.current.get(p.id);
            if (existingNode) {
                // Mutate existing object so ForceGraph picks up the changes on next repaint without a structural reset
                existingNode.status = p.status;
                if (existingNode.name !== p.name) existingNode.name = p.name;
            } else {
                const newNode = { id: p.id, group: "peer", status: p.status, name: p.name, currentRadius: 0 };
                prevNodesMap.current.set(p.id, newNode);
                nodesRef.current.push(newNode);
                structuralChange = true;
            }

            // Guarantee a link exists for this peer
            let existingLink = prevLinksMap.current.get(p.id);
            if (!existingLink) {
                const newLink = { source: "self", target: p.id };
                prevLinksMap.current.set(p.id, newLink);
                linksRef.current.push(newLink);
                structuralChange = true;
            }
        });

        // Prune stale nodes by mutating original array
        for (let i = nodesRef.current.length - 1; i >= 0; i--) {
            const n = nodesRef.current[i];
            if (!activeIds.has(n.id)) {
                nodesRef.current.splice(i, 1);
                prevNodesMap.current.delete(n.id);
                structuralChange = true;
            }
        }

        // Prune stale links by mutating original array
        for (let i = linksRef.current.length - 1; i >= 0; i--) {
            const l = linksRef.current[i];
            const targetId = typeof l.target === 'object' ? l.target.id : l.target;
            if (!activeIds.has(targetId)) {
                linksRef.current.splice(i, 1);
                prevLinksMap.current.delete(targetId);
                structuralChange = true;
            }
        }

        // ONLY update React state if physical topology changed. 
        // Eliminates D3 reheats (twitches) on every interval tick.
        if (structuralChange) {
            setGraphDataProp({ nodes: [...nodesRef.current], links: [...linksRef.current] });
        }
    }, [peers]);

    // Format ID to first...last
    const formatId = (id: string) => {
        if (!id) return '';
        if (id === 'self') return 'Self';
        if (id.length <= 12) return id;
        return `${id.substring(0, 6)}...${id.substring(id.length - 4)}`;
    };

    const handleNodeClick = (node: any) => {
        const id = node.id === 'self' ? info?.node_id : node.id;
        if (id) {
            setSelectedNode(id);
            // Scroll to item in list
            if (itemRefs.current[id]) {
                itemRefs.current[id]?.scrollIntoView({ behavior: 'smooth', block: 'center' });
            }
        }
    };

    return (
        <div className="h-full flex flex-col pt-4 animate-in fade-in duration-300">
            <header className="mb-6 flex justify-between items-end border-b border-bbs-panel pb-2">
                <div>
                    <h2 className="text-2xl font-bold uppercase tracking-widest text-bbs-green drop-shadow-[0_0_8px_var(--color-bbs-green-dim)] flex items-center space-x-2">
                        <MapIcon size={24} />
                        <span>Mesh Topology</span>
                    </h2>
                    <p className="text-bbs-muted text-sm mt-1">Peer Discovery & Routing Visualization</p>
                </div>
                <div className="text-xs flex space-x-4">
                    <span className="px-2 py-1 border border-bbs-amber text-bbs-amber bg-bbs-amber/10">DHT: Bootstrap</span>
                    <span className="px-2 py-1 border border-bbs-cyan text-bbs-cyan bg-bbs-cyan/10">mDNS: Active</span>
                </div>
            </header>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 h-[calc(100%-80px)]">
                {/* Abstract Map Area */}
                <div ref={containerRef} className="border border-bbs-panel bg-bbs-surface/40 relative overflow-hidden flex items-center justify-center min-h-[300px]">
                    {dimensions.width > 0 && (
                        <ForceGraph2D
                            ref={fgRef}
                            width={dimensions.width}
                            height={dimensions.height}
                            graphData={graphDataProp}
                            nodeAutoColorBy="group"
                            nodeRelSize={6}
                            linkColor={() => "rgba(42, 196, 171, 0.3)"} // cyan-ish lines
                            onZoom={() => lastInteractionTime.current = Date.now()}
                            onNodeDragEnd={() => lastInteractionTime.current = Date.now()}
                            onNodeHover={(node) => setHoverNode(node)}
                            onNodeClick={handleNodeClick}
                            nodeCanvasObjectMode={() => "replace"} // completely override default node drawing
                            nodeCanvasObject={(node, ctx, globalScale) => {
                                const isSelf = node.id === 'self';
                                const peerData = isSelf ? null : peers.find(p => p.id === node.id);
                                const isOffline = peerData && peerData.status === 'Offline';
                                const isSelected = node.id === selectedNode || (isSelf && selectedNode === info?.node_id);
                                const isHovered = hoverNode === node;
                                let label = formatId(node.id as string);
                                if (isSelf && info && info.node_name) {
                                    label = info.node_name;
                                } else if (peerData && peerData.name) {
                                    label = peerData.name;
                                }
                                const fontSize = 12 / globalScale;

                                // Animated Radius Pop-in/out
                                const targetRadius = isHovered ? 6 : (isSelected ? 5 : 4);
                                node.currentRadius = node.currentRadius ?? 0;
                                node.currentRadius += (targetRadius - node.currentRadius) * 0.15; // Smooth 60fps native easing interpolation
                                const r = Math.max(0.1, node.currentRadius);

                                // Node Circle
                                ctx.beginPath();
                                ctx.arc(node.x || 0, node.y || 0, r, 0, 2 * Math.PI, false);

                                if (isSelf) {
                                    ctx.fillStyle = '#2ac4ab'; // cyan
                                } else if (isOffline) {
                                    ctx.fillStyle = '#52525b'; // zinc-600 / grey
                                } else {
                                    ctx.fillStyle = '#d27d2d'; // amber
                                }
                                ctx.fill();

                                // Selection ring
                                if (isSelected) {
                                    ctx.strokeStyle = '#2ac4ab';
                                    ctx.lineWidth = 1.5;
                                    ctx.stroke();
                                }

                                // Basic Label text
                                ctx.font = `${fontSize}px Sans-Serif`;
                                ctx.textAlign = 'center';
                                ctx.textBaseline = 'middle';
                                ctx.fillStyle = isOffline ? '#71717a' : '#a1a1aa';
                                ctx.fillText(label, node.x || 0, (node.y || 0) + (isHovered ? 12 : 8));

                                // Hover Tooltip
                                if (isHovered) {
                                    const tooltipText = isSelf
                                        ? `Self: ${info?.node_id ? formatId(info.node_id) : 'Unknown'}`
                                        : `RTT: ${peerData?.latency || 'N/A'} | ${peerData?.route || 'Unknown'}`;

                                    const textWidth = ctx.measureText(tooltipText).width;
                                    const bckgDimensions = [textWidth, fontSize].map(n => n + fontSize * 0.8) as [number, number];

                                    ctx.fillStyle = 'rgba(0, 0, 0, 0.85)';
                                    ctx.fillRect((node.x || 0) - bckgDimensions[0] / 2, (node.y || 0) - bckgDimensions[1] - 8, bckgDimensions[0], bckgDimensions[1]);

                                    ctx.strokeStyle = '#2ac4ab';
                                    ctx.lineWidth = 0.5;
                                    ctx.strokeRect((node.x || 0) - bckgDimensions[0] / 2, (node.y || 0) - bckgDimensions[1] - 8, bckgDimensions[0], bckgDimensions[1]);

                                    ctx.textAlign = 'center';
                                    ctx.textBaseline = 'middle';
                                    ctx.fillStyle = '#2ac4ab';
                                    ctx.fillText(tooltipText, node.x || 0, (node.y || 0) - bckgDimensions[1] / 2 - 8);
                                }
                            }}
                            d3AlphaDecay={0.02}
                            d3VelocityDecay={0.3}
                        />
                    )}
                </div>

                {/* Peer List */}
                <div className="border border-bbs-panel bg-black flex flex-col overflow-hidden">
                    <div className="bg-bbs-panel/30 p-2 text-xs font-bold uppercase tracking-widest text-bbs-muted border-b border-bbs-panel flex justify-between">
                        <span>Known Routing Table</span>
                        <span>Count: {peers.length}</span>
                    </div>
                    <div className="p-2 space-y-2 overflow-y-auto custom-scrollbar" ref={listRef}>
                        {peers.map((p, i) => (
                            <div
                                key={i}
                                ref={el => { itemRefs.current[p.id] = el; }}
                                onClick={() => setSelectedNode(p.id)}
                                className={`p-3 border cursor-pointer transition-colors grid grid-cols-2 gap-2 text-sm ${selectedNode === p.id ? 'border-bbs-cyan bg-bbs-cyan/10' : 'border-bbs-panel/50 hover:border-bbs-green/50 hover:bg-bbs-green/5'}`}
                            >
                                <div
                                    className="col-span-2 text-bbs-text font-mono text-xs opacity-80 border-b border-bbs-panel/30 pb-1 mb-1 flex items-center justify-between cursor-pointer group hover:text-white transition-colors"
                                    onClick={() => handleCopy(p.id)}
                                    title="Click to copy full Peer ID"
                                >
                                    <span className="truncate w-10/12">
                                        {p.name ? <><span className="text-bbs-amber font-bold">{p.name}</span> <span className="opacity-50">({formatId(p.id)})</span></> : p.id}
                                    </span>
                                    <span className="text-bbs-muted group-hover:text-bbs-green transition-colors w-4 h-4 flex-shrink-0">
                                        {copiedId === p.id ? <CheckCircle2 size={14} className="text-bbs-green" /> : <Copy size={14} />}
                                    </span>
                                </div>
                                <div className="flex items-center space-x-2 text-bbs-muted">
                                    <div className={`w-1.5 h-1.5 rounded-full ${p.status === 'Active' ? 'bg-bbs-green shadow-[0_0_3px_var(--color-bbs-green)]' : 'bg-bbs-muted'}`} />
                                    <span className="uppercase text-xs">{p.status}</span>
                                </div>
                                <div className="text-right text-bbs-muted text-xs font-mono">{p.latency}</div>
                                <div className="flex items-center space-x-1 text-xs text-bbs-amber">
                                    {p.route.includes('WiFi') ? <Wifi size={10} /> : <Radio size={10} />}
                                    <span>{p.route}</span>
                                </div>
                                <div className="flex justify-end items-center space-x-1 text-xs text-bbs-cyan">
                                    <Lock size={10} />
                                    <span className="uppercase">{p.trust}</span>
                                </div>
                            </div>
                        ))}
                    </div>
                </div>
            </div>
        </div>
    );
}
