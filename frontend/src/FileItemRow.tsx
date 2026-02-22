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

export const FileItemRow = ({ item, onDelete, onUpload, onDownload, focusKeyPrefix, status, isDisabled = false }: FileItemRowProps) => {
    const { ref: downloadRef, focused: downloadFocused } = useFocusable({
        focusKey: onDownload && !isDisabled ? `${focusKeyPrefix}-download` : undefined,
        onEnterPress: onDownload,
        onArrowPress: (direction: string) => {
            if (direction === 'up') {
                return false;
            }
            if (direction === 'right' && onDelete) {
                setFocus(`${focusKeyPrefix}-delete`);
                return false;
            }
            return true;
        }
    });

    const { ref: uploadRef, focused: uploadFocused } = useFocusable({
        focusKey: onUpload && !isDisabled ? `${focusKeyPrefix}-upload` : undefined,
        onEnterPress: onUpload,
        onArrowPress: (direction: string) => {
            if (direction === 'up') {
                return false;
            }
            if (direction === 'right' && onDelete) {
                setFocus(`${focusKeyPrefix}-delete`);
                return false;
            }
            return true;
        }
    });

    const { ref: deleteRef, focused: deleteFocused } = useFocusable({
        focusKey: onDelete && !isDisabled ? `${focusKeyPrefix}-delete` : undefined,
        onEnterPress: onDelete,
        onArrowPress: (direction: string) => {
            if (direction === 'left') {
                if (onUpload) setFocus(`${focusKeyPrefix}-upload`);
                else if (onDownload) setFocus(`${focusKeyPrefix}-download`);
                return false;
            }
            return true;
        }
    });

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
                {onDownload && (
                    <button
                        className={`file-action-btn file-download-btn ${downloadFocused ? 'focused' : ''} ${isDisabled ? 'disabled' : ''}`}
                        ref={downloadRef}
                        disabled={isDisabled}
                        onClick={(e) => {
                            e.stopPropagation();
                            if (!isDisabled) onDownload();
                        }}
                        title="Download from RomM"
                    >
                        <DownloadIcon size={16} />
                    </button>
                )}
                {onUpload && (
                    <button
                        className={`file-action-btn file-upload-btn ${uploadFocused ? 'focused' : ''} ${isDisabled ? 'disabled' : ''}`}
                        ref={uploadRef}
                        disabled={isDisabled}
                        onClick={(e) => {
                            e.stopPropagation();
                            if (!isDisabled) onUpload();
                        }}
                        title="Upload save to RomM"
                    >
                        <UploadIcon size={16} />
                    </button>
                )}
                {onDelete && (
                    <button
                        className={`file-action-btn file-delete-btn ${deleteFocused ? 'focused' : ''} ${isDisabled ? 'disabled' : ''}`}
                        ref={deleteRef}
                        disabled={isDisabled}
                        onClick={(e) => {
                            e.stopPropagation();
                            if (!isDisabled) onDelete();
                        }}
                        title="Delete locally"
                    >
                        <TrashIcon size={16} />
                    </button>
                )}
            </div>
        </div>
    );
};
