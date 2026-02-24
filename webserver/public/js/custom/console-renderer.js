var ConsoleRenderer = {

    badgeClasses: {
        'info':        'badge-info',
        'up':          'badge-info',
        'join':        'badge-success',
        'success':     'badge-success',
        'downlink':    'badge-primary',
        'ack':         'badge-secondary',
        'mac_command': 'badge-warning',
        'warn':        'badge-warning',
        'error':       'badge-danger',
        'status':      'badge-light'
    },

    normalize: function(rawEvent, source) {
        var e = { time: null, type: 'info', message: '', detail: null };

        if (source === 'system') {
            e.time = rawEvent.time ? new Date(rawEvent.time) : new Date();
            e.type = rawEvent.isError ? 'error' : 'info';
            e.message = rawEvent.type + ': ' + rawEvent.message;
        } else if (source === 'device') {
            e.time = rawEvent.time ? new Date(rawEvent.time) : new Date();
            e.type = rawEvent.type || 'info';
            var summary = rawEvent.type || '';
            if (rawEvent.fCnt !== undefined && rawEvent.fCnt !== null) summary += ' FCnt=' + rawEvent.fCnt;
            if (rawEvent.fPort !== undefined && rawEvent.fPort !== null) summary += ' FPort=' + rawEvent.fPort;
            e.message = summary;
            e.detail = rawEvent;
        } else if (source === 'codec') {
            e.time = new Date();
            e.type = rawEvent.type || 'info';
            e.message = rawEvent.message || '';
        }

        return e;
    },

    renderEntry: function(event, opts) {
        opts = opts || {};
        var badgeClass = this.badgeClasses[event.type] || 'badge-dark';
        var timeStr = event.time ? event.time.toLocaleTimeString() : '';

        var textClass = opts.darkBg ? 'text-light' : '';
        var mutedClass = opts.darkBg ? 'text-white-50' : 'text-muted';

        var html = '';
        if (event.detail) {
            html += '<div class="device-event-row mb-1 p-2 border-bottom">';
        } else {
            html += '<div class="p-1">';
        }

        html += '<span class="badge ' + badgeClass + '">' + this.escapeHtml(event.type) + '</span> ';
        html += '<small class="' + mutedClass + '">' + timeStr + '</small> ';
        html += '<span class="' + textClass + '">' + this.escapeHtml(event.message) + '</span>';

        if (event.detail) {
            html += '<pre class="collapse mt-1 mb-0" style="font-size:11px">' +
                    this.escapeHtml(JSON.stringify(event.detail, null, 2)) + '</pre>';
        }

        html += '</div>';
        return html;
    },

    append: function(containerId, event, opts) {
        var container = document.getElementById(containerId);
        if (!container) return;

        var placeholder = container.querySelector('.text-muted');
        if (placeholder && container.children.length === 1) placeholder.remove();

        container.insertAdjacentHTML('beforeend', this.renderEntry(event, opts));
        container.scrollTop = container.scrollHeight;
    },

    clear: function(containerId, placeholderText) {
        var container = document.getElementById(containerId);
        if (!container) return;
        container.innerHTML = '<div class="text-muted">' + (placeholderText || '') + '</div>';
    },

    escapeHtml: function(str) {
        var div = document.createElement('div');
        div.appendChild(document.createTextNode(str));
        return div.innerHTML;
    }
};

var DeviceEventManager = {
    currentDevEUI: null,
    subscribers: {},

    subscribe: function(devEUI, containerId) {
        if (this.currentDevEUI && this.currentDevEUI !== devEUI) {
            var oldContainers = Object.keys(this.subscribers);
            for (var i = 0; i < oldContainers.length; i++) {
                ConsoleRenderer.clear(oldContainers[i]);
            }
            this.subscribers = {};
        }

        if (this.currentDevEUI !== devEUI) {
            if (this.currentDevEUI) {
                socket.emit('stop-device-events', { devEUI: this.currentDevEUI });
            }
            this.currentDevEUI = devEUI;
            socket.emit('stream-device-events', { devEUI: devEUI });
        }

        this.subscribers[containerId] = true;
        ConsoleRenderer.clear(containerId);
    },

    unsubscribe: function(containerId) {
        delete this.subscribers[containerId];

        if (Object.keys(this.subscribers).length === 0 && this.currentDevEUI) {
            socket.emit('stop-device-events', { devEUI: this.currentDevEUI });
            this.currentDevEUI = null;
        }
    },

    unsubscribeAll: function() {
        this.subscribers = {};
        if (this.currentDevEUI) {
            socket.emit('stop-device-events', { devEUI: this.currentDevEUI });
            this.currentDevEUI = null;
        }
    },

    onEvent: function(event) {
        var normalized = ConsoleRenderer.normalize(event, 'device');
        var containers = Object.keys(this.subscribers);
        for (var i = 0; i < containers.length; i++) {
            ConsoleRenderer.append(containers[i], normalized);
        }
    }
};
