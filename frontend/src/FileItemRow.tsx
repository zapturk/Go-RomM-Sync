import { useFocusable, setFocus } from '@noriginmedia/norigin-spatial-navigation';
import { types } from "../wailsjs/go/models";
import { TrashIcon, UploadIcon, DownloadIcon } from "./components/Icons";

export type FileItemRowType = types.FileItem | types.ServerSave | types.ServerState;

interface FileItemRowProps {
    item: FileItemRowType;
    onDelete?: () => void;
    onUpload?: () => void;
    onDownload?: () => void;
    focusKeyPrefix: string;
    status?: 'newer' | 'older' | 'equal' | 'unsynced';
    isDisabled?: boolean;
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

export const FileItemRow = ({ item, onDelete, onUpload, onDownload, focusKeyPrefix, status, isDisabled = false }: FileItemRowProps) => {
    // Provide a default focus receiver if the prefix is targeted directly
    const { ref: rowRef } = useFocusable({
        focusKey: !isDisabled ? focusKeyPrefix : undefined,
        onFocus: () => {
            if (onUpload) setFocus(`${focusKeyPrefix}-upload`);
            else if (onDownload) setFocus(`${focusKeyPrefix}-download`);
            else if (onDelete) setFocus(`${focusKeyPrefix}-delete`);
        }
    });

    const fileName = (item as types.FileItem).name || (item as types.ServerSave).file_name;
    const coreName = (item as types.FileItem).core || (item as types.ServerSave).emulator;

    return (
        <div className={`file-item-row ${isDisabled ? 'disabled' : ''}`} ref={rowRef}>
            <span className="file-name" title={fileName}>{fileName}</span>
            {status && (
                <span className={`file-status ${status}`}>
                    {status === 'equal' ? 'synced' : status}
                </span>
            )}
            <span className="file-core">{coreName}</span>
            <div className={`file-item-actions ${isDisabled ? 'hidden' : ''}`}>
                {onDownload && !isDisabled && (
                    <ActionButton
                        focusKey={`${focusKeyPrefix}-download`}
                        onEnterPress={onDownload}
                        className="file-action-btn file-download-btn"
                        title="Download from RomM"
                        onArrowPress={(direction) => {
                            if (direction === 'up') return false;
                            if (direction === 'right' && onDelete) {
                                setFocus(`${focusKeyPrefix}-delete`);
                                return false;
                            }
                            return true;
                        }}
                    >
                        <DownloadIcon size={16} />
                    </ActionButton>
                )}
                {onUpload && !isDisabled && (
                    <ActionButton
                        focusKey={`${focusKeyPrefix}-upload`}
                        onEnterPress={onUpload}
                        className="file-action-btn file-upload-btn"
                        title="Upload save to RomM"
                        onArrowPress={(direction) => {
                            if (direction === 'up') return false;
                            if (direction === 'right' && onDelete) {
                                setFocus(`${focusKeyPrefix}-delete`);
                                return false;
                            }
                            return true;
                        }}
                    >
                        <UploadIcon size={16} />
                    </ActionButton>
                )}
                {onDelete && !isDisabled && (
                    <ActionButton
                        focusKey={`${focusKeyPrefix}-delete`}
                        onEnterPress={onDelete}
                        className="file-action-btn file-delete-btn"
                        title="Delete locally"
                        onArrowPress={(direction) => {
                            if (direction === 'left') {
                                if (onUpload) setFocus(`${focusKeyPrefix}-upload`);
                                else if (onDownload) setFocus(`${focusKeyPrefix}-download`);
                                return false;
                            }
                            return true;
                        }}
                    >
                        <TrashIcon size={16} />
                    </ActionButton>
                )}
            </div>
        </div>
    );
};
