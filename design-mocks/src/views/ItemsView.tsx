import { MOCK_ITEMS } from "@/data/mock"
import { ItemsPanel } from "@/components/ItemsPanel"

interface ItemsViewProps {
  onItemClick: (id: string) => void
  onAddItem: () => void
}

export function ItemsView({ onItemClick, onAddItem }: ItemsViewProps) {
  return (
    <div className="p-6 max-w-5xl mx-auto w-full">
      <ItemsPanel
        items={MOCK_ITEMS}
        onItemClick={onItemClick}
        onAddItem={onAddItem}
        title="All Items"
        subtitle={`${MOCK_ITEMS.length} items across all locations`}
        showStats
      />
    </div>
  )
}
