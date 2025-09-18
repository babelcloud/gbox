import React from 'react';
import styles from './ControlButtons.module.css';

interface ControlButtonsProps {
  onAction: (action: string) => void;
  onIMESwitch?: () => void;
  onDisconnect?: () => void;
  isVisible?: boolean;
  onToggleVisibility?: () => void;
  showDisconnect?: boolean;
}

export const ControlButtons: React.FC<ControlButtonsProps> = ({ onAction, onIMESwitch, isVisible = true, onToggleVisibility }) => {
  const buttons = [
    { id: 'power', title: 'Power', icon: PowerIcon },
    { id: 'volume_up', title: 'Volume Up', icon: VolumeUpIcon },
    { id: 'volume_down', title: 'Volume Down', icon: VolumeDownIcon },
    { id: 'separator', isSeparator: true },
    { id: 'back', title: 'Back', icon: BackIcon },
    { id: 'home', title: 'Home', icon: HomeIcon },
    { id: 'app_switch', title: 'Recent Apps', icon: RecentIcon },
    { id: 'separator2', isSeparator: true },
    { id: 'ime_switch', title: 'Switch Input Method', icon: IMESwitchIcon, isIMESwitch: true },
  ];

  return (
    <div className={styles.controlButtons}>
      {buttons.map((button) => {
        if (button.isSeparator) {
          return <div key={button.id} className={styles.separator} />;
        }
        
        const Icon = button.icon;
        const handleClick = () => {
          if (button.isIMESwitch && onIMESwitch) {
            onIMESwitch();
          } else if (button.isToggle && onToggleVisibility) {
            onToggleVisibility();
          } else {
            onAction(button.id);
          }
        };
        
        return (
          <button
            key={button.id}
            className={styles.controlBtn}
            title={button.title}
            onMouseDown={handleClick}
            onTouchStart={(e) => {
              e.preventDefault();
              handleClick();
            }}
          >
            {Icon && <Icon />}
          </button>
        );
      })}
    </div>
  );
};

// Icon components
const PowerIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 48 48">
    <path fill="none" d="M0 0h48v48H0z"/>
    <path d="M26 6h-4v20h4V6zm9.67 4.33l-2.83 2.83C35.98 15.73 38 19.62 38 24c0 7.73-6.27 14-14 14s-14-6.27-14-14c0-4.38 2.02-8.27 5.16-10.84l-2.83-2.83C8.47 13.63 6 18.52 6 24c0 9.94 8.06 18 18 18s18-8.06 18-18c0-5.48-2.47-10.37-6.33-13.67z" fill="currentColor"/>
  </svg>
);

const VolumeUpIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 48 48">
    <path d="M6 18v12h8l10 10V8L14 18H6zm27 6c0-3.53-2.04-6.58-5-8.05v16.11c2.96-1.48 5-4.53 5-8.06zM28 6.46v4.13c5.78 1.72 10 7.07 10 13.41s-4.22 11.69-10 13.41v4.13c8.01-1.82 14-8.97 14-17.54S36.01 8.28 28 6.46z" fill="currentColor"/>
    <path d="M0 0h48v48H0z" fill="none"/>
  </svg>
);

const VolumeDownIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 48 48">
    <path d="M37 24c0-3.53-2.04-6.58-5-8.05v16.11c2.96-1.48 5-4.53 5-8.06zm-27-6v12h8l10 10V8L18 18h-8z" fill="currentColor"/>
    <path d="M0 0h48v48H0z" fill="none"/>
  </svg>
);

const BackIcon = () => (
  <svg width="48" height="48" viewBox="0 0 48 48" xmlns="http://www.w3.org/2000/svg">
    <rect x="0" y="0" width="48" height="48" fill="none"/>
    <path d="M36.7088473,10.9494765 L36.7088478,37.6349688 C36.7088478,39.4039498 35.4820844,40.0949115 33.9646508,39.1757647 L12.1373795,25.9544497 C10.6218013,25.0364267 10.6199459,23.5491414 12.1373794,22.6299946 L33.9646503,9.40868054 C35.4802284,8.49065763 36.7088473,9.16835511 36.7088473,10.9494765 Z M33.5088482,13.4305237 L33.5088482,35.1305245 L15.5088482,24.2805241 L33.5088482,13.4305237 Z" fill="currentColor"/>
  </svg>
);

const HomeIcon = () => (
  <svg width="48" height="48" viewBox="0 0 48 48" xmlns="http://www.w3.org/2000/svg">
    <rect x="0" y="0" width="48" height="48" fill="none"/>
    <path d="M24,35.2 L24,35.2 C30.1855892,35.2 35.2,30.1855892 35.2,24 C35.2,17.8144108 30.1855892,12.8 24,12.8 C17.8144108,12.8 12.8,17.8144108 12.8,24 C12.8,30.1855892 17.8144108,35.2 24,35.2 L24,35.2 Z M24,38.4 L24,38.4 C16.0470996,38.4 9.6,31.9529004 9.6,24 C9.6,16.0470996 16.0470996,9.6 24,9.6 C31.9529004,9.6 38.4,16.0470996 38.4,24 C38.4,31.9529004 31.9529004,38.4 24,38.4 L24,38.4 Z" fill="currentColor"/>
  </svg>
);

const RecentIcon = () => (
  <svg width="48" height="48" viewBox="0 0 48 48" xmlns="http://www.w3.org/2000/svg">
    <rect x="0" y="0" width="48" height="48" fill="none"/>
    <path d="M12.7921429,12.8 L12.7921429,12.8 C12.7959945,12.8 12.8,12.7959954 12.8,12.7921429 L12.8,35.2078571 C12.8,35.2040055 12.7959954,35.2 12.7921429,35.2 L35.2078571,35.2 C35.2040055,35.2 35.2,35.2040046 35.2,35.2078571 L35.2,12.7921429 C35.2,12.7959945 35.2040046,12.8 35.2078571,12.8 L12.7921429,12.8 Z M12.7921429,9.6 L12.7921429,9.6 L35.2078571,9.6 C36.9718035,9.6 38.4,11.029171 38.4,12.7921429 L38.4,35.2078571 C38.4,36.9718035 36.970829,38.4 35.2078571,38.4 L12.7921429,38.4 C11.0281965,38.4 9.6,36.970829 9.6,35.2078571 L9.6,12.7921429 C9.6,11.0281965 11.029171,9.6 12.7921429,9.6 L12.7921429,9.6 Z" fill="currentColor"/>
  </svg>
);

const IMESwitchIcon = () => (
  <svg width="48" height="48" viewBox="0 0 48 48" xmlns="http://www.w3.org/2000/svg">
    <rect x="0" y="0" width="48" height="48" fill="none"/>
    {/* Globe circle */}
    <circle cx="24" cy="24" r="20" fill="none" stroke="currentColor" strokeWidth="2"/>
    {/* Horizontal lines */}
    <path d="M8 24h32" stroke="currentColor" strokeWidth="1.5"/>
    <path d="M12 18c0-3.3 2.7-6 6-6s6 2.7 6 6" stroke="currentColor" strokeWidth="1.5" fill="none"/>
    <path d="M12 30c0 3.3 2.7 6 6 6s6-2.7 6-6" stroke="currentColor" strokeWidth="1.5" fill="none"/>
    <path d="M24 18c0-3.3 2.7-6 6-6s6 2.7 6 6" stroke="currentColor" strokeWidth="1.5" fill="none"/>
    <path d="M24 30c0 3.3 2.7 6 6 6s6-2.7 6-6" stroke="currentColor" strokeWidth="1.5" fill="none"/>
    {/* Vertical line */}
    <path d="M24 4v40" stroke="currentColor" strokeWidth="1.5"/>
  </svg>
);

const HideIcon = () => (
  <svg width="48" height="48" viewBox="0 0 48 48" xmlns="http://www.w3.org/2000/svg">
    <rect x="0" y="0" width="48" height="48" fill="none"/>
    <path d="M12 20c0-2 1-4 2-6l-2-2-2 2 2 2c-1 2-2 4-2 6s1 4 2 6l-2 2 2 2 2-2c1-2 2-4 2-6z" fill="currentColor"/>
    <path d="M36 20c0 2-1 4-2 6l2 2 2-2-2-2c1-2 2-4 2-6s-1-4-2-6l2-2-2-2-2 2c-1 2-2 4-2 6z" fill="currentColor"/>
    <path d="M24 8c-4 0-8 2-12 6l2 2c3-3 6-4 10-4s7 1 10 4l2-2c-4-4-8-6-12-6z" fill="currentColor"/>
    <path d="M24 32c4 0 8-2 12-6l-2-2c-3 3-6 4-10 4s-7-1-10-4l-2 2c4 4 8 6 12 6z" fill="currentColor"/>
    <path d="M20 24c0-2 2-4 4-4s4 2 4 4-2 4-4 4-4-2-4-4z" fill="currentColor"/>
  </svg>
);

const ShowIcon = () => (
  <svg width="48" height="48" viewBox="0 0 48 48" xmlns="http://www.w3.org/2000/svg">
    <rect x="0" y="0" width="48" height="48" fill="none"/>
    <path d="M24 8c-4 0-8 2-12 6l2 2c3-3 6-4 10-4s7 1 10 4l2-2c-4-4-8-6-12-6z" fill="currentColor"/>
    <path d="M24 32c4 0 8-2 12-6l-2-2c-3 3-6 4-10 4s-7-1-10-4l-2 2c4 4 8 6 12 6z" fill="currentColor"/>
    <path d="M20 24c0-2 2-4 4-4s4 2 4 4-2 4-4 4-4-2-4-4z" fill="currentColor"/>
  </svg>
);
