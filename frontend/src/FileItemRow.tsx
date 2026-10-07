import { useFocusable, setFocus } from '@noriginmedia/norigin-spatial-navigation';
import { types } from "../wailsjs/go/models";
import { TrashIcon, UploadIcon, DownloadIcon, SaveIcon, StateIcon } from "./components/Icons";
import { TIMESTAMP_REGEX } from "./constants";

type FileItemRowType = types.FileItem | types.ServerSave | types.ServerState;

interface FileItemRowProps {
    item: FileItemRowType;
    itemType?: 'save' | 'state';
    onDelete?: () => void;
    onUpload?: () => void;
    onDownload?: () => void;
    focusKeyPrefix: string;
    status?: 'newer' | 'older' | 'equal' | 'unsynced';
    isDisabled?: boolean;
    isOffline?: boolean;
    onArrowLeft?: () => void;
    onArrowUp?: () => void;
}

interface ActionButtonProps {
    focusKey: string;
    onEnterPress: () => void;
    onArrowPress?: (direction: string) => boolean;
    className: string;
    children: React.ReactNode;
    title: string;
}

const ActionButton = ({ focusKey, onEnterPress, onArrowPress, className, children, title }: ActionButtonProps) => {
    const { ref, focused } = useFocusable({
        focusKey,
        onEnterPress,
        onArrowPress
    });

    return (
        <button
            ref={ref}
            type="button"
            className={`${className} ${focused ? 'focused' : ''}`}
            onClick={(e) => {
                e.stopPropagation();
                onEnterPress();
            }}
            title={title}
        >
            {children}
        </button>
    );
};

interface ActionBtnProps {
    focusKeyPrefix: string;
    onAction: () => void;
    onDelete?: () => void;
    onUpload?: () => void;
    onDownload?: () => void;
    onArrowLeft?: () => void;
    onArrowUp?: () => void;
}

const DownloadButton = ({ focusKeyPrefix, onAction, onDelete, onArrowLeft, onArrowUp }: ActionBtnProps) => {
    const handleArrowPress = (direction: string) => {
        if (direction === 'up' && onArrowUp) {
            onArrowUp();
            return false;
        }
        if (direction === 'left' && onArrowLeft) {
            onArrowLeft();
            return false;
        }
        if (direction === 'right' && onDelete) {
            setFocus(`${focusKeyPrefix}-delete`);
            return false;
        }
        return true;
    };

    return (
        <ActionButton
            focusKey={`${focusKeyPrefix}-download`}
            onEnterPress={onAction}
            className="file-action-btn file-download-btn"
            title="Download from RomM"
            onArrowPress={handleArrowPress}
        >
            <DownloadIcon size={16} />
        </ActionButton>
    );
};

const UploadButton = ({ focusKeyPrefix, onAction, onDelete, onArrowLeft, onArrowUp }: ActionBtnProps) => {
    const handleArrowPress = (direction: string) => {
        if (direction === 'up' && onArrowUp) {
            onArrowUp();
            return false;
        }
        if (direction === 'left' && onArrowLeft) {
            onArrowLeft();
            return false;
        }
        if (direction === 'right' && onDelete) {
            setFocus(`${focusKeyPrefix}-delete`);
            return false;
        }
        return true;
    };

    return (
        <ActionButton
            focusKey={`${focusKeyPrefix}-upload`}
            onEnterPress={onAction}
            className="file-action-btn file-upload-btn"
            title="Upload save to RomM"
            onArrowPress={handleArrowPress}
        >
            <UploadIcon size={16} />
        </ActionButton>
    );
};

const DeleteButton = ({ focusKeyPrefix, onAction, onUpload, onDownload, onArrowLeft, onArrowUp }: ActionBtnProps) => {
    const handleArrowPress = (direction: string) => {
        if (direction === 'up' && onArrowUp) {
            onArrowUp();
            return false;
        }
        if (direction === 'left') {
            if (onUpload) {
                setFocus(`${focusKeyPrefix}-upload`);
                return false;
            }
            if (onDownload) {
                setFocus(`${focusKeyPrefix}-download`);
                return false;
            }
            if (onArrowLeft) {
                onArrowLeft();
                return false;
            }
        }
        return true;
    };

    return (
        <ActionButton
            focusKey={`${focusKeyPrefix}-delete`}
            onEnterPress={onAction}
            className="file-action-btn file-delete-btn"
            title="Delete"
            onArrowPress={handleArrowPress}
        >
            <TrashIcon size={16} />
        </ActionButton>
    );
};

export const getItemName = (item: any) => {
    const raw = item?.name || item?.file_name || '';
    return raw.replace(TIMESTAMP_REGEX, '');
};

export const getItemCore = (item: any) => item?.core || item?.emulator || '';

export const formatItemDate = (dateStr?: string) => {
    if (!dateStr) return '';
    try {
        const d = new Date(dateStr);
        if (isNaN(d.getTime())) return '';
        return d.toLocaleDateString(undefined, {
            month: 'short',
            day: 'numeric',
            year: 'numeric',
            hour: 'numeric',
            minute: '2-digit'
        });
    } catch {
        return '';
    }
};

export const formatItemSize = (bytes?: number) => {
    if (!bytes || bytes <= 0) return '';
    const KB = 1024;
    const MB = KB * 1024;
    const GB = MB * 1024;
    if (bytes < MB) return `${(bytes / KB).toFixed(1)} KB`;
    if (bytes < GB) return `${(bytes / MB).toFixed(2)} MB`;
    return `${(bytes / GB).toFixed(2)} GB`;
};

const getFocusTarget = (onUpload: any, onDownload: any, onDelete: any, prefix: string) => {
    if (onUpload) return `${prefix}-upload`;
    if (onDownload) return `${prefix}-download`;
    if (onDelete) return `${prefix}-delete`;
    return undefined;
};

const FileStatusBadge = ({ status }: { status?: 'newer' | 'older' | 'equal' | 'unsynced' }) => {
    if (!status) return null;
    const text = status === 'equal' ? 'synced' : status;
    return <span className={`file-status ${status}`}>{text}</span>;
};

interface FileItemRowActionsProps {
    isDisabled: boolean;
    isOffline: boolean;
    focusKeyPrefix: string;
    onDownload?: () => void;
    onUpload?: () => void;
    onDelete?: () => void;
    onArrowLeft?: () => void;
    onArrowUp?: () => void;
}

const FileItemRowActions = ({
    isDisabled,
    isOffline,
    focusKeyPrefix,
    onDownload,
    onUpload,
    onDelete,
    onArrowLeft,
    onArrowUp
}: FileItemRowActionsProps) => {
    if (isDisabled) return null;
    return (
        <div className="file-item-actions">
            {onDownload && (
                <DownloadButton
                    focusKeyPrefix={focusKeyPrefix}
                    onAction={onDownload}
                    onDelete={onDelete}
                    onArrowLeft={onArrowLeft}
                    onArrowUp={onArrowUp}
                />
            )}
            {onUpload && !isOffline && (
                <UploadButton
                    focusKeyPrefix={focusKeyPrefix}
                    onAction={onUpload}
                    onDelete={onDelete}
                    onArrowLeft={onArrowLeft}
                    onArrowUp={onArrowUp}
                />
            )}
            {onDelete && (
                <DeleteButton
                    focusKeyPrefix={focusKeyPrefix}
                    onAction={onDelete}
                    onUpload={isOffline ? undefined : onUpload}
                    onDownload={onDownload}
                    onArrowLeft={onArrowLeft}
                    onArrowUp={onArrowUp}
                />
            )}
        </div>
    );
};

export const FileItemRow = ({
    item,
    itemType = 'save',
    onDelete,
    onUpload,
    onDownload,
    focusKeyPrefix,
    status,
    isDisabled = false,
    isOffline = false,
    onArrowLeft,
    onArrowUp
}: FileItemRowProps) => {
    const { ref: rowRef } = useFocusable({
        focusKey: isDisabled ? undefined : focusKeyPrefix,
        onFocus: () => {
            const target = getFocusTarget(isOffline ? undefined : onUpload, onDownload, onDelete, focusKeyPrefix);
            if (target) setFocus(target);
        }
    });

    const fileName = getItemName(item);
    const coreName = getItemCore(item);
    const dateText = formatItemDate((item as any)?.updated_at);
    const sizeText = formatItemSize((item as any)?.file_size_bytes);
    const rowClassName = `file-item-row ${isDisabled ? 'disabled' : ''}`;

    return (
        <div className={rowClassName} ref={rowRef}>
            <div className="file-item-icon">
                {itemType === 'save' ? <SaveIcon size={16} /> : <StateIcon size={16} />}
            </div>
            <div className="file-item-details">
                <span className="file-name" title={fileName}>{fileName}</span>
                {(dateText || sizeText) && (
                    <div className="file-item-subline">
                        {dateText && <span className="file-meta-date">{dateText}</span>}
                        {dateText && sizeText && <span className="file-meta-sep">•</span>}
                        {sizeText && <span className="file-meta-size">{sizeText}</span>}
                    </div>
                )}
            </div>
            <div className="file-item-badges">
                <FileStatusBadge status={status} />
                {coreName && <span className="file-core">{coreName}</span>}
            </div>
            <FileItemRowActions
                isDisabled={isDisabled}
                isOffline={isOffline}
                focusKeyPrefix={focusKeyPrefix}
                onDownload={onDownload}
                onUpload={onUpload}
                onDelete={onDelete}
                onArrowLeft={onArrowLeft}
                onArrowUp={onArrowUp}
            />
        </div>
    );
};
