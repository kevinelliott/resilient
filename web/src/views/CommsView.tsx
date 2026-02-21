import { Hash, User, Terminal, Send, Database, Folder, File, Download } from 'lucide-react';
import { useState, useEffect, useRef } from 'react';

type SocialMessage = {
    id: string;
    topic: string;
    author_id: string;
    content: string;
    ref_target_id: string;
    created_at: number;
};

export function CommsView() {
    const [messages, setMessages] = useState<SocialMessage[]>([]);
    const [input, setInput] = useState('');
    const [targetPeer, setTargetPeer] = useState<string | null>(null);
    const messagesEndRef = useRef<HTMLDivElement>(null);

    const fetchMessages = () => {
        const url = targetPeer ? `/api/dm?peer_id=${targetPeer}` : '/api/chat';
        fetch(url)
            .then(res => res.json())
            .then(data => setMessages(data || []))
            .catch(err => console.error("Failed to fetch messages", err));
    };

    useEffect(() => {
        fetchMessages();
        const interval = setInterval(fetchMessages, 2000);
        return () => clearInterval(interval);
    }, [targetPeer]);

    useEffect(() => {
        messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    }, [messages]);

    const handleSend = () => {
        if (!input.trim()) return;

        let endpoint = '/api/chat';
        let bodyData: any = { content: input, ref_target_id: '' };

        // Support command-line style "/dm Qm1234..." switches
        let actualContent = input;
        if (input.startsWith('/dm ')) {
            const parts = input.split(' ');
            if (parts.length >= 3) {
                const pID = parts[1];
                setTargetPeer(pID);
                endpoint = '/api/dm';
                actualContent = parts.slice(2).join(' ');
                bodyData = { content: actualContent, recipient_id: pID };
            }
        } else if (targetPeer) {
            endpoint = '/api/dm';
            bodyData = { content: actualContent, recipient_id: targetPeer };
        }

        fetch(endpoint, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(bodyData),
        })
            .then(() => {
                setInput('');
                fetchMessages();
            })
            .catch(err => console.error("Failed to send message", err));
    };

    const handleKeyDown = (e: React.KeyboardEvent) => {
        if (e.key === 'Enter') handleSend();
    };

    const handleResolvePayload = (type: string, id: string) => {
        // In a real implementation this would trigger a contextual modal or auto-fetch
        fetch('/api/fetch', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ file_id: id }) // simplified
        }).then(() => alert(`Triggered resolution for ${type} ${id}`))
            .catch(err => alert("Failed to resolve payload: " + err));
    };

    const renderContent = (text: string) => {
        const regex = /res:\/\/(catalog|bundle|file)\/([a-zA-Z0-9_\-]+)/g;
        const parts = [];
        let lastIndex = 0;
        let match;

        while ((match = regex.exec(text)) !== null) {
            // Push pre-text
            if (match.index > lastIndex) {
                parts.push(<span key={`text-${lastIndex}`}>{text.substring(lastIndex, match.index)}</span>);
            }

            const type = match[1];
            const id = match[2];

            let Icon = File;
            if (type === 'catalog') Icon = Database;
            if (type === 'bundle') Icon = Folder;

            parts.push(
                <div key={`res-${match.index}`} className="my-2 inline-flex items-center space-x-3 bg-black/40 border border-bbs-panel p-2 rounded-sm group hover:border-bbs-amber transition-colors cursor-pointer w-full max-w-sm" onClick={() => handleResolvePayload(type, id)}>
                    <div className="w-8 h-8 flex items-center justify-center bg-bbs-surface rounded-sm group-hover:bg-bbs-amber/10">
                        <Icon size={16} className="text-bbs-amber group-hover:animate-pulse" />
                    </div>
                    <div className="flex-1 min-w-0">
                        <div className="text-xs font-bold text-white uppercase tracking-wider truncate">{type} Payload</div>
                        <div className="text-[10px] text-bbs-muted font-mono truncate">{id}</div>
                    </div>
                    <button className="text-bbs-cyan hover:text-white p-1 rounded-sm border border-bbs-cyan/30 hover:bg-bbs-cyan/20 transition-all shadow-[0_0_5px_var(--color-bbs-cyan)]">
                        <Download size={14} />
                    </button>
                </div>
            );

            lastIndex = regex.lastIndex;
        }

        // Push post-text
        if (lastIndex < text.length) {
            parts.push(<span key={`text-${lastIndex}`}>{text.substring(lastIndex)}</span>);
        }

        return parts.length > 0 ? parts : text;
    };

    return (
        <div className="flex flex-col h-full animate-in fade-in duration-300">
            <header className="mb-4 flex justify-between items-end border-b border-bbs-panel pb-2">
                <div>
                    <h2 className="text-2xl font-bold uppercase tracking-widest text-[#ff3333] drop-shadow-[0_0_8px_rgba(255,51,51,0.4)] flex items-center space-x-2">
                        <Terminal size={24} />
                        <span>Secure Comms</span>
                    </h2>
                    <p className="text-bbs-muted text-sm mt-1">Gossipsub Realtime Mesh Network</p>
                </div>
                <div className="text-xs text-bbs-muted flex items-center space-x-4">
                    <span className="flex items-center space-x-1"><div className="w-2 h-2 bg-bbs-green rounded-full shadow-[0_0_5px_var(--color-bbs-green)]" /> <span>Encrypted</span></span>
                    {targetPeer ? (
                        <div className="flex items-center space-x-2">
                            <span className="text-bbs-amber flex items-center space-x-1">
                                <User size={12} /> <span>P2P: {targetPeer.substring(0, 8)}...</span>
                            </span>
                            <button onClick={() => setTargetPeer(null)} className="text-[10px] hover:text-bbs-red underline ml-2">Back to Global</button>
                        </div>
                    ) : (
                        <span className="flex items-center space-x-1 text-bbs-cyan text-opacity-80"><Hash size={12} /> <span>#global-mesh</span></span>
                    )}
                </div>
            </header>

            <div className="flex-1 border border-bbs-panel bg-bbs-surface/50 shadow-inner flex flex-col p-4 overflow-hidden rounded-sm relative selection:bg-bbs-red/30">

                {/* Messages */}
                <div className="flex-1 overflow-y-auto space-y-4 pr-2 custom-scrollbar flex flex-col-reverse">
                    <div className="space-y-4 pb-2">
                        {messages.length === 0 && (
                            <div className="text-bbs-muted text-xs italic text-center opacity-50">No messages in local orbit yet...</div>
                        )}
                        {messages.map(msg => {
                            const date = new Date(msg.created_at * 1000);
                            const timeStr = `${date.getHours().toString().padStart(2, '0')}:${date.getMinutes().toString().padStart(2, '0')}`;
                            const isSelf = msg.author_id.startsWith('Qm'); // Simplistic check for testing
                            return (
                                <div key={msg.id} className="flex flex-col group mt-4">
                                    <div className="flex items-center space-x-2 mb-1">
                                        <span className="text-[10px] text-bbs-muted font-mono self-end mb-[1px] opacity-70">[{timeStr}]</span>
                                        <span className={`font-bold text-sm tracking-wide ${isSelf ? 'text-bbs-cyan' : 'text-bbs-amber'}`}>
                                            <User size={12} className="inline mr-1 pb-1" />
                                            {msg.author_id.substring(0, 8)}...
                                        </span>
                                    </div>
                                    <div className="pl-[52px] text-sm break-words leading-relaxed text-[#e0e0e0] flex flex-col items-start whitespace-pre-wrap">
                                        {renderContent(msg.content)}
                                    </div>
                                </div>
                            );
                        })}
                        <div ref={messagesEndRef} />
                    </div>
                </div>

                {/* Input area */}
                <div className="mt-4 pt-4 border-t border-bbs-panel/50 flex space-x-2 relative z-10">
                    <div className="flex-1 relative">
                        <span className="absolute left-3 top-2.5 text-bbs-amber font-bold animate-pulse">&gt;</span>
                        <input
                            type="text"
                            value={input}
                            onChange={(e) => setInput(e.target.value)}
                            onKeyDown={handleKeyDown}
                            className="w-full bg-black border border-bbs-panel focus:border-bbs-amber focus:ring-1 focus:ring-bbs-amber outline-none px-8 py-2 text-sm text-bbs-text placeholder-bbs-muted/50 rounded-sm transition-all shadow-[inset_0_0_10px_rgba(0,0,0,0.8)]"
                            placeholder={targetPeer ? `Secure DM to ${targetPeer.substring(0, 8)}...` : "Transmit global mesh message... (or type '/dm <peerID> <msg>' to whisper)"}
                        />
                    </div>
                    <button onClick={handleSend} className="bbs-button bg-black hover:bg-bbs-amber hover:text-black">
                        <Send size={16} />
                    </button>
                </div>
            </div>
        </div>
    );
}
