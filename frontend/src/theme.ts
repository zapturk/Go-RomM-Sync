export interface BackgroundPreset {
    id: string;
    name: string;
    css: string;
    preview: string;
    description: string;
}

export interface FontOption {
    id: string;
    name: string;
    family: string;
    previewText: string;
    description: string;
}

export const BACKGROUND_PRESETS: BackgroundPreset[] = [
    {
        id: 'cosmic-purple',
        name: 'Cosmic Purple',
        css: 'radial-gradient(circle at top left, #5d3fd3, #2a1b5e) #553e98',
        preview: 'radial-gradient(circle at top left, #5d3fd3, #2a1b5e)',
        description: 'Vibrant violet gaming gradient'
    },
    {
        id: 'oled-midnight',
        name: 'OLED Midnight',
        css: 'radial-gradient(circle at top left, #181924, #0a0b0f) #0d0e12',
        preview: 'radial-gradient(circle at top left, #181924, #0a0b0f)',
        description: 'Deep onyx black for OLED displays'
    },
    {
        id: 'cyber-blue',
        name: 'Cyber Blue',
        css: 'radial-gradient(circle at top left, #0e4b75, #081b33) #071526',
        preview: 'radial-gradient(circle at top left, #0e4b75, #081b33)',
        description: 'Techy electric blue & deep navy'
    },
    {
        id: 'forest-emerald',
        name: 'Forest Emerald',
        css: 'radial-gradient(circle at top left, #0d5c3a, #072618) #062014',
        preview: 'radial-gradient(circle at top left, #0d5c3a, #072618)',
        description: 'Lush organic deep emerald'
    },
    {
        id: 'crimson-noir',
        name: 'Crimson Noir',
        css: 'radial-gradient(circle at top left, #731a26, #24070a) #1a0507',
        preview: 'radial-gradient(circle at top left, #731a26, #24070a)',
        description: 'Intense arcade ruby & scarlet'
    },
    {
        id: 'synthwave-sunset',
        name: 'Synthwave',
        css: 'linear-gradient(135deg, #3b0764 0%, #831843 50%, #9a3412 100%)',
        preview: 'linear-gradient(135deg, #3b0764 0%, #831843 50%, #9a3412 100%)',
        description: '80s retro neon sunset vibe'
    }
];

export const FONT_OPTIONS: FontOption[] = [
    {
        id: 'orbitron',
        name: 'Orbitron',
        family: "'Orbitron', sans-serif",
        previewText: 'Orbitron',
        description: 'Futuristic sci-fi gamer'
    },
    {
        id: 'press-start',
        name: 'Press Start 2P',
        family: "'Press Start 2P', monospace",
        previewText: '8-BIT RETRO',
        description: 'Authentic arcade & pixel'
    },
    {
        id: 'inter',
        name: 'Inter',
        family: "'Inter', sans-serif",
        previewText: 'Inter Modern',
        description: 'Crisp, ultra-clean UI'
    },
    {
        id: 'roboto',
        name: 'Roboto',
        family: "'Roboto', sans-serif",
        previewText: 'Roboto UI',
        description: 'Smooth, balanced sans'
    },
    {
        id: 'nunito',
        name: 'Nunito',
        family: "'Nunito', sans-serif",
        previewText: 'Nunito Soft',
        description: 'Friendly rounded sans'
    },
    {
        id: 'system',
        name: 'System Default',
        family: 'system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
        previewText: 'System OS',
        description: 'Native platform font'
    }
];

export interface ColorPreset {
    id: string;
    name: string;
    color: string;
}

export const TEXT_COLOR_PRESETS: ColorPreset[] = [
    { id: 'white', name: 'Pure White', color: '#ffffff' },
    { id: 'cyan', name: 'Cyber Cyan', color: '#00e5ff' },
    { id: 'green', name: 'Neon Green', color: '#00ff66' },
    { id: 'amber', name: 'Arcade Gold', color: '#ffb703' },
    { id: 'pink', name: 'Synth Pink', color: '#ff3399' },
    { id: 'lavender', name: 'Lavender', color: '#d8b4fe' },
    { id: 'mint', name: 'Mint Teal', color: '#5eead4' }
];

export function getComputedBackground(backgroundId: string, customBg: string): string {
    if (backgroundId === 'custom-color') {
        return customBg || '#121212';
    }
    if (backgroundId === 'custom-image') {
        if (!customBg) {
            return BACKGROUND_PRESETS[0].css;
        }
        let imgUrl = customBg;
        if (!customBg.startsWith('http://') && !customBg.startsWith('https://') && !customBg.startsWith('data:')) {
            imgUrl = `/custom-background?path=${encodeURIComponent(customBg)}`;
        }
        return `linear-gradient(rgba(0, 0, 0, 0.65), rgba(0, 0, 0, 0.65)), url("${imgUrl}")`;
    }
    const preset = BACKGROUND_PRESETS.find(p => p.id === backgroundId) || BACKGROUND_PRESETS[0];
    return preset.css;
}

export function applyTheme(backgroundId: string, customBg: string, fontId: string, textColor?: string) {
    const root = document.documentElement;
    const body = document.body;

    // Font
    const font = FONT_OPTIONS.find(f => f.id === fontId) || FONT_OPTIONS[0];
    root.style.setProperty('--app-font', font.family);
    body.setAttribute('data-font', font.id);

    // Font / Text Color
    const activeColor = textColor || '#ffffff';
    root.style.setProperty('--app-text-color', activeColor);

    // Background
    const bgCss = getComputedBackground(backgroundId, customBg);
    root.style.setProperty('--app-background', bgCss);

    // Persist to local cache for instant initial rendering
    localStorage.setItem('theme_background', backgroundId || 'cosmic-purple');
    localStorage.setItem('theme_custom_background', customBg || '');
    localStorage.setItem('theme_font', fontId || 'orbitron');
    localStorage.setItem('theme_text_color', activeColor);
}

export function initThemeFromStorage() {
    const bg = localStorage.getItem('theme_background') || 'cosmic-purple';
    const customBg = localStorage.getItem('theme_custom_background') || '';
    const font = localStorage.getItem('theme_font') || 'orbitron';
    const textColor = localStorage.getItem('theme_text_color') || '#ffffff';
    applyTheme(bg, customBg, font, textColor);
}
