document.addEventListener("DOMContentLoaded", function () {
	const eventSource = new EventSource('/.flogo/events');
	
	// Handle connection open
	eventSource.onopen = function() {
		console.log('flogo: SSE connection established');
		document.getElementById('status').textContent = 'Connected to server';
	};
	
	// Handle messages (events without a specific type)
	eventSource.onmessage = function(event) {
		console.log('flogo: Received message:', event.data);
	};
	
	// Handle "connected" event specifically
	eventSource.addEventListener('connected', function(event) {
		console.log('flogo: Connected event received:', event.data);
	});
	
	// Handle errors
	eventSource.onerror = function(error) {
		console.error('flogo: SSE error:', error);
		//document.getElementById('status').textContent = 'Connection error - check console';
	};
});
