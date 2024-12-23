function registerRemoteComponent(app, context) {
    return app.component("remote", {
        // show a dropdown of garage doors
        // show the current state of the selected garage door
        // show push button to open/close garage door
        // push button is locked to prevent accidental click
        // user can unlock the push button by clicking on the lock switch
        template: `
<div class="container mt-5">
    <div class="card">
        <div class="card-header">
            <div class="row">
                <div class="col"></div>
                <div class="col-auto">
                    <select class="form-control" id="garageDoorSelect" v-model="selectedGarageDoorId">
                        <option v-for="door in garageDoors" :key="door.getId()" :value="door.getId()">
                            {{ door.getName() }}
                        </option>
                    </select>
                </div>
                <div class="col-auto">
                    <div class="form-check form-switch mt-2">
                        <input type="checkbox" class="form-check-input" id="lockSwitch" v-model="isButtonLocked" />
                        <label class="form-check-label" for="lockSwitch">
                        </label>
                    </div>
                </div>
                <div class="col"></div>
            </div>
        </div>
        <div class="card-body">
            <div class="garage">
                <button type="button" class="btn btn-outline-light btn-lg garage-inner" :disabled="isButtonLocked" @click="onDoorButtonPressed">
                    {{nextGarageStatus}} <div v-if="isButtonLocked" style="display:inline;">(<i class="fa fa-lock"></i>)</div>
                </button>                
                <button type="button" class="garage-inner-fill btn btn-secondary btn-lg" :style="garageStyle" :disabled="true">
                </button>
            </div>
        </div>
    </div>
</div>`,
        data() {
            qEntityStore
                .getEventManager()
                .addEventListener(Q_STORE_EVENTS.CONNECTED, this.onStoreConnected.bind(this))
                .addEventListener(Q_STORE_EVENTS.DISCONNECTED, this.onStoreDisconnected.bind(this));

            return {
                
                garageDoors: [],
                selectedGarageDoorId: "",
                isButtonLocked: true,
                notificationTokens: [],
                percentClosed: 0,
                lastGarageStatus: ""
            };
        },

        computed: {
            garageStatus() {
                if( this.percentClosed === 100 ) {
                    return "Closed"
                } else if ( this.percentClosed === 0 ) {
                    return "Opened";
                } else if ( this.lastGarageStatus === "Closed" ) {
                    return "Opening";
                } else if ( this.lastGarageStatus === "Opened" ) {
                    return "Closing";
                } else {
                    return "Partially Opened";
                }
            },

            nextGarageStatus() {
                if ( this.percentClosed === 100 ) {
                    return "Open";
                } else if ( this.percentClosed === 0 ) {
                    return "Close";
                } else if ( this.lastGarageStatus === "Closed" ) {
                    return "Stop Opening";
                } else if ( this.lastGarageStatus === "Opened" ) {
                    return "Stop Closing";
                } else {
                    return "Open / Close";
                }
            },

            doorButtonDisabled() {
                return this.isButtonLocked || this.garageDoors.length === 0 || !this.selectedGarageDoorId || !this.garageDoors[this.selectedGarageDoorId];
            },

            garageStyle() {
                return {
                    height: this.percentClosed + "%",
                    transition: "height 0.5s linear"
                }
            }
        },
        
        mounted() {
            if (qEntityStore.isConnected()) {
                this.onStoreConnected();
            }
        },

        methods: {
            onStoreConnected() {
                
                
                qEntityStore
                    .queryAllEntities("GarageDoor")
                    .then(event => this.onQueryAllEntities(event))
                    .catch(error => qError(`[Remote::onDatabaseConnected] ${error}`));
            },

            onStoreDisconnected() {
                
            },

            onDoorButtonPressed() {
                this.isButtonLocked = true;
                
                const value = new proto.protobufs.Int();
                value.setRaw(0);
                const valueAsAny = new proto.google.protobuf.Any();
                valueAsAny.pack(value.serializeBinary(), qMessageType(value));

                qEntityStore.write([{
                    id: this.selectedGarageDoorId,
                    field: "ToggleTrigger",
                    value: valueAsAny
                }]).catch(error => qError(`[Remote::onDoorButtonPressed] ${error}`));
            },

            onQueryAllEntities(result) {
                this.garageDoors = result["entities"];
                this.selectedGarageDoorId = this.garageDoors[0].getId();
            },

            onRegisterNotification(event) {
                this.notificationTokens = event.tokens;
            },

            onNotification(event) {
                const notification = event.getCurrent();
                const protoClass = notification.getValue().getTypeName().split('.').reduce((o,i)=> o[i], proto);
                this.percentClosed = protoClass.deserializeBinary(notification.getValue().getValue_asU8()).getRaw();

                if ( this.garageStatus === "Closed" || this.garageStatus === "Opened" ) {
                    this.lastGarageStatus = this.garageStatus;
                }
            },

            onReadResult(event) {
                const protoClass = event[0].getValue().getTypeName().split('.').reduce((o,i)=> o[i], proto);
                this.percentClosed = protoClass.deserializeBinary(event[0].getValue().getValue_asU8()).getRaw();
            }
        },
        
        watch: {
            selectedGarageDoorId: function(newVal) {
                if (this.notificationTokens.length > 0) {
                    qEntityStore
                        .unregisterNotifications(this.notificationTokens.slice())
                        .then(() => this.notificationTokens = [])
                        .catch(error => qError(`[Remote::selectedGarageDoorId] ${error}`));
                }

                qEntityStore
                    .registerNotifications([
                        { id: newVal, field: "PercentClosed", notifyOnChange: true }
                    ], this.onNotification.bind(this))
                    .then(event => this.onRegisterNotification(event))
                    .catch(error => qError(`[Remote::selectedGarageDoorId] ${error}`));

                qEntityStore
                    .read([
                        { id: newVal, field: "PercentClosed" }
                    ])
                    .then(event => this.onReadResult(event))
                    .catch(error => qError(`[Remote::selectedGarageDoorId] ${error}`));
            },
        }
    })
}