import React from 'react';
import { AndroidLiveView } from './AndroidLiveView';
import { LiveViewProps } from '../types';

/**
 * Generic LiveView component that can be extended for different device types
 */
export const LiveView: React.FC<LiveViewProps> = (props) => {
  // For now, default to AndroidLiveView
  // In the future, this could detect device type and render appropriate view
  return <AndroidLiveView {...props} />;
};