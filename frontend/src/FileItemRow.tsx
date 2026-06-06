import { useFocusable, setFocus } from '@noriginmedia/norigin-spatial-navigation';
import { types } from "../wailsjs/go/models";
import { TrashIcon, UploadIcon, DownloadIcon } from "./components/Icons";

type FileItemRowType = types.FileItem | types.ServerSave | types.ServerState;

interface FileItemRowProps {
    item: FileItemRowType;
    onDelete?: () => void;
    onUpload?: () => void;
    onDownload?: () => void;
    focusKeyPrefix: string;
    status?: 'newer' | 'older' | 'equal' | 'unsynced';
    isDisabled?: boolean;
    isOffline?: boolean;
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
}

const DownloadButton = ({ focusKeyPrefix, onAction, onDelete }: ActionBtnProps) => {
    const handleArrowPress = (direction: string) => {
        if (direction === 'up') return false;
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

const UploadButton = ({ focusKeyPrefix, onAction, onDelete }: ActionBtnProps) => {
    const handleArrowPress = (direction: string) => {
        if (direction === 'up') return false;
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

const DeleteButton = ({ focusKeyPrefix, onAction, onUpload, onDownload }: ActionBtnProps) => {
    const handleArrowPress = (direction: string) => {
        if (direction === 'left') {
            if (onUpload) {
                setFocus(`${focusKeyPrefix}-upload`);
                return false;
            }
            if (onDownload) {
                setFocus(`${focusKeyPrefix}-download`);
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
            title="Delete locally"
            onArrowPress={handleArrowPress}
        >
            <TrashIcon size={16} />
        </ActionButton>
    );
};

const getItemName = (item: any) => item.name || item.file_name;
const getItemCore = (item: any) => item.core || item.emulator;

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
}

const FileItemRowActions = ({
    isDisabled,
    isOffline,
    focusKeyPrefix,
    onDownload,
    onUpload,
    onDelete
}: FileItemRowActionsProps) => {
    if (isDisabled) return null;
    return (
        <div className="file-item-actions">
            {onDownload && (
                <DownloadButton
                    focusKeyPrefix={focusKeyPrefix}
                    onAction={onDownload}
                    onDelete={onDelete}
                />
            )}
            {onUpload && !isOffline && (
                <UploadButton
                    focusKeyPrefix={focusKeyPrefix}
                    onAction={onUpload}
                    onDelete={onDelete}
                />
            )}
            {onDelete && (
                <DeleteButton
                    focusKeyPrefix={focusKeyPrefix}
                    onAction={onDelete}
                    onUpload={isOffline ? undefined : onUpload}
                    onDownload={onDownload}
                />
            )}
        </div>
    );
};

export const FileItemRow = ({ item, onDelete, onUpload, onDownload, focusKeyPrefix, status, isDisabled = false, isOffline = false }: FileItemRowProps) => {
    const { ref: rowRef } = useFocusable({
        focusKey: isDisabled ? undefined : focusKeyPrefix,
        onFocus: () => {
            const target = getFocusTarget(isOffline ? undefined : onUpload, onDownload, onDelete, focusKeyPrefix);
            if (target) setFocus(target);
        }
    });

    const fileName = getItemName(item);
    const coreName = getItemCore(item);
    const rowClassName = `file-item-row ${isDisabled ? 'disabled' : ''}`;

    return (
        <div className={rowClassName} ref={rowRef}>
            <span className="file-name" title={fileName}>{fileName}</span>
            <FileStatusBadge status={status} />
            <span className="file-core">{coreName}</span>
            <FileItemRowActions
                isDisabled={isDisabled}
                isOffline={isOffline}
                focusKeyPrefix={focusKeyPrefix}
                onDownload={onDownload}
                onUpload={onUpload}
                onDelete={onDelete}
            />
        </div>
    );
};
