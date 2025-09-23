import React from 'react';
import { AndroidLiveView } from './AndroidLiveView';
import { LiveViewProps } from '../types';

/**
 * Generic LiveView component that can be extended for different device types
 * Now uses the refactored implementation by default
 */
export const LiveView: React.FC<LiveViewProps> = (props) => {
  // Use the refactored AndroidLiveView by default
  // The refactored version provides better architecture and maintainability
  return <AndroidLiveView {...props} />;
};