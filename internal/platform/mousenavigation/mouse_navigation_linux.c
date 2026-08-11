//go:build linux && desktop

#include "mouse_navigation_linux.h"

#include <gtk/gtk.h>
#include <webkit2/webkit2.h>

extern void emitMouseNavigation(int direction);

static GtkWidget *find_webview(GtkWidget *widget) {
    if (WEBKIT_IS_WEB_VIEW(widget)) {
        return widget;
    }
    if (!GTK_IS_CONTAINER(widget)) {
        return NULL;
    }

    GList *children = gtk_container_get_children(GTK_CONTAINER(widget));
    GtkWidget *result = NULL;
    for (GList *child = children; child != NULL && result == NULL; child = child->next) {
        result = find_webview(GTK_WIDGET(child->data));
    }
    g_list_free(children);
    return result;
}

static gboolean handle_mouse_button(GtkWidget *widget, GdkEventButton *event, gpointer data) {
    (void)widget;
    (void)data;

    if (event == NULL || event->type != GDK_BUTTON_PRESS) {
        return FALSE;
    }
    if (event->button == 8) {
        emitMouseNavigation(-1);
        return TRUE;
    }
    if (event->button == 9) {
        emitMouseNavigation(1);
        return TRUE;
    }
    return FALSE;
}

static gboolean connect_mouse_handler(gpointer data) {
    (void)data;

    GList *windows = gtk_window_list_toplevels();
    for (GList *window = windows; window != NULL; window = window->next) {
        GtkWidget *webview = find_webview(GTK_WIDGET(window->data));
        if (webview != NULL) {
            g_signal_connect(webview, "button-press-event", G_CALLBACK(handle_mouse_button), NULL);
            break;
        }
    }
    g_list_free(windows);
    return G_SOURCE_REMOVE;
}

void installMouseNavigationHandler(void) {
    g_idle_add(connect_mouse_handler, NULL);
}
