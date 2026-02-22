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
}

export const FileItemRow = ({ item, onDelete, onUpload, onDownload, focusKeyPrefix }: FileItemRowProps) => {
    const { ref: downloadRef, focused: downloadFocused } = useFocusable({
        focusKey: onDownload ? `${focusKeyPrefix}-download` : undefined,
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
        focusKey: onUpload ? `${focusKeyPrefix}-upload` : undefined,
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
        focusKey: onDelete ? `${focusKeyPrefix}-delete` : undefined,
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
        focusKey: focusKeyPrefix,
        onFocus: () => {
            if (onUpload) setFocus(`${focusKeyPrefix}-upload`);
            else if (onDownload) setFocus(`${focusKeyPrefix}-download`);
            else if (onDelete) setFocus(`${focusKeyPrefix}-delete`);
        }
    });

    const fileName = (item as types.FileItem).name || (item as types.ServerSave).file_name;
    const coreName = (item as types.FileItem).core || (item as types.ServerSave).emulator;

    return (
        <div className="file-item-row" ref={rowRef}>
            <span className="file-name" title={fileName}>{fileName}</span>
            <span className="file-core">{coreName}</span>
            <div className="file-item-actions">
                {onDownload && (
                    <button
                        className={`file-action-btn file-download-btn ${downloadFocused ? 'focused' : ''}`}
                        ref={downloadRef}
                        onClick={(e) => {
                            e.stopPropagation();
                            onDownload();
                        }}
                        title="Download from RomM"
                    >
                        <DownloadIcon size={16} />
                    </button>
                )}
                {onUpload && (
                    <button
                        className={`file-action-btn file-upload-btn ${uploadFocused ? 'focused' : ''}`}
                        ref={uploadRef}
                        onClick={(e) => {
                            e.stopPropagation();
                            onUpload();
                        }}
                        title="Upload save to RomM"
                    >
                        <UploadIcon size={16} />
                    </button>
                )}
                {onDelete && (
                    <button
                        className={`file-action-btn file-delete-btn ${deleteFocused ? 'focused' : ''}`}
                        ref={deleteRef}
                        onClick={(e) => {
                            e.stopPropagation();
                            onDelete();
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
