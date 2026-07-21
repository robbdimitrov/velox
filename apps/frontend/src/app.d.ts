declare global {
  namespace App {
    interface Locals {
      user: {
        id: string;
        email: string;
        role: 'reserver' | 'organizer' | 'admin';
        name?: string;
      } | null;
    }
    interface PageData {
      gatewayBaseURL?: string;
      user?: App.Locals['user'];
    }
  }
}

export {};
