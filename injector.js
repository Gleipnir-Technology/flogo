// Status Display Class
class StatusDisplay {
	constructor(element) {
		if (!element) {
			throw new Error("StatusDisplay requires a valid HTML element");
		}

		this.container = element;
		this.STATUS = {
			FINE: "fine",
			BUILDING: "building",
			ERROR: "error",
		};

		this.COLORS = {
			ERROR: "#ffebee",
			ERROR_TEXT: "#c62828",
			BUILDING: "#fff9c4",
			BUILDING_TEXT: "#f57f17",
		};

		this.init();
	}

	init() {
		// Create the HTML structure
		this.container.innerHTML = `
			<div id="status-bar" style="
				position: fixed;
				bottom: 0;
				left: 0;
				right: 0;
				padding: 12px 20px;
				font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
				font-size: 14px;
				box-shadow: 0 -2px 10px rgba(0,0,0,0.1);
				transition: all 0.3s ease;
				z-index: 9999;
				display: none;
			">
				<div style="display: flex; justify-content: space-between; align-items: center;">
					<span id="status-message"></span>
					<button id="status-close" style="
						background: none;
						border: none;
						font-size: 20px;
						cursor: pointer;
						opacity: 0.6;
						padding: 0 5px;
						line-height: 1;
					">×</button>
				</div>
			</div>
			
			<div id="error-detail" style="
				position: fixed;
				bottom: 60px;
				left: 20px;
				right: 20px;
				max-height: 400px;
				background: white;
				border: 2px solid ${this.COLORS.ERROR_TEXT};
				border-radius: 8px;
				padding: 20px;
				font-family: 'Courier New', monospace;
				font-size: 12px;
				overflow: auto;
				box-shadow: 0 4px 20px rgba(0,0,0,0.2);
				z-index: 9998;
				display: none;
			">
				<h3 style="margin: 0 0 10px 0; color: ${this.COLORS.ERROR_TEXT};">Error Details</h3>
				<pre id="error-stack" style="
					margin: 0;
					white-space: pre-wrap;
					word-wrap: break-word;
					color: #333;
				"></pre>
			</div>
		`;

		// Cache DOM elements
		this.statusBar = this.container.querySelector("#status-bar");
		this.statusMessage = this.container.querySelector("#status-message");
		this.statusClose = this.container.querySelector("#status-close");
		this.errorDetail = this.container.querySelector("#error-detail");
		this.errorStack = this.container.querySelector("#error-stack");

		// Bind event listeners
		this.statusClose.addEventListener("click", () => {
			this.hide();
		});
	}

	setStatus(status, message, stackTrace) {
		if (status === this.STATUS.FINE) {
			this.statusBar.style.display = "none";
			this.errorDetail.style.display = "none";
		} else if (status === this.STATUS.BUILDING) {
			this.statusBar.style.display = "block";
			this.statusBar.style.background = this.COLORS.BUILDING;
			this.statusBar.style.color = this.COLORS.BUILDING_TEXT;
			this.statusMessage.textContent = message || "⚙️ Building...";
			this.errorDetail.style.display = "none";
		} else if (status === this.STATUS.ERROR) {
			this.statusBar.style.display = "block";
			this.statusBar.style.background = this.COLORS.ERROR;
			this.statusBar.style.color = this.COLORS.ERROR_TEXT;
			this.statusMessage.textContent = message || "❌ Error occurred";

			if (stackTrace) {
				this.errorDetail.style.display = "block";
				this.errorStack.textContent = stackTrace;
			} else {
				this.errorDetail.style.display = "none";
			}
		}
	}

	showBuilding(message) {
		this.setStatus(this.STATUS.BUILDING, message);
	}

	showError(message, error) {
		const stackTrace = error ? error.stack || error.toString() : null;
		this.setStatus(this.STATUS.ERROR, message, stackTrace);
	}

	hide() {
		this.setStatus(this.STATUS.FINE);
	}

	destroy() {
		// Clean up event listeners and DOM
		this.statusClose.removeEventListener("click", this.hide);
		this.container.innerHTML = "";
	}
}

document.addEventListener("DOMContentLoaded", function () {
	const flogoElement = document.getElementById("flogo");
	const statusDisplay = new StatusDisplay(flogoElement);

	let eventSource = null;
	let retryCount = 0;
	let retryTimeout = null;
	const baseDelay = 1000; // Start at 1 second
	const maxDelay = 30000; // Cap at 30 seconds
	const maxRetries = Infinity; // Or set a limit like 10

	function connect() {
		// Clean up existing connection
		if (eventSource) {
			eventSource.close();
		}

		statusDisplay.showBuilding("Compiling...");
		eventSource = new EventSource("/.flogo/events");

		eventSource.onopen = function () {
			console.log("flogo: SSE connection established");
			retryCount = 0; // Reset on successful connection
		};

		eventSource.onmessage = function (event) {
			console.log("flogo: Received message:", event.data);
		};

		eventSource.addEventListener("connected", function (event) {
			console.log("flogo: Connected event received:", event.data);
		});

		eventSource.onerror = function (error) {
			console.error("flogo: SSE error:", error);
			eventSource.close();
			reconnect();
		};
	}

	function reconnect() {
		// Clear any existing retry timeout
		if (retryTimeout) {
			clearTimeout(retryTimeout);
		}

		// Check if max retries reached (if you want a limit)
		if (retryCount >= maxRetries) {
			console.error("flogo: Max retries reached");
			statusDisplay.showBuilding("Connection failed");
			return;
		}

		retryCount++;

		// Calculate delay: 1s, 2s, 4s, 8s, 16s, 30s (max)
		const delay = Math.min(baseDelay * Math.pow(2, retryCount - 1), maxDelay);

		console.log(`flogo: Reconnecting in ${delay}ms (attempt ${retryCount})`);
		statusDisplay.showBuilding(`Reconnecting in ${delay / 1000}s...`);

		retryTimeout = setTimeout(() => {
			connect();
		}, delay);
	}

	// Initial connection
	connect();

	// Optional: Clean up on page unload
	window.addEventListener("beforeunload", function () {
		if (retryTimeout) clearTimeout(retryTimeout);
		if (eventSource) eventSource.close();
	});
});
