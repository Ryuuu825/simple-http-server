// File watcher client - connects to SSE endpoint for live reload
(function() {
    'use strict';
    
    const eventSource = new EventSource('/__sse');
    
    eventSource.onopen = function() {
        console.log('Connected to file watcher');
    };
    
    eventSource.onmessage = function(event) {
        console.log('File change detected:', event.data);
        if (event.data && event.data !== 'Connected to file watcher') {
            // Reload the page when a file change is detected
            setTimeout(() => {
                window.location.reload();
            }, 300);
        }
    };
    
    eventSource.onerror = function(error) {
        console.error('File watcher error:', error);
        eventSource.close();
        
        // Attempt to reconnect after 5 seconds
        setTimeout(() => {
            window.location.reload();
        }, 5000);
    };
    
    // Clean up on page unload
    window.addEventListener('beforeunload', function() {
        eventSource.close();
    });
})();
