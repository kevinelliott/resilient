import { useState } from 'react';
import { Copy, Check } from 'lucide-react';

export function CopyableID({ value, label }: { value: string, label?: string }) {
    const [copied, setCopied] = useState(false);

    const handleCopy = (e: React.MouseEvent) => {
        e.stopPropagation();
        navigator.clipboard.writeText(value);
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
    };

    if (!value) return null;

    return (
        <button
            onClick={handleCopy}
            className="group flex items-center space-x-1 font-mono text-[10px] text-bbs-muted hover:text-bbs-cyan transition-colors cursor-pointer max-w-full text-left bg-transparent border-none p-0 w-full"
            title={`Copy ${label || 'ID'}: ${value}`}
        >
            {label && <span className="opacity-60 shrink-0">{label}:</span>}
            <span className="truncate">{value}</span>
            <span className="shrink-0 transition-opacity ml-1">
                {copied ? <Check size={10} className="text-bbs-green" /> : <Copy size={10} className="opacity-0 group-hover:opacity-100" />}
            </span>
        </button>
    );
}
