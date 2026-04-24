/**
 * FontAwesome → Lucide bridge.
 *
 * During the Epic #1324 migration, legacy code still references
 * FontAwesome icons by name. New code must import directly from
 * `lucide-vue-next` (see devdocs/frontend/icons.md). This module lets
 * transitional code swap FontAwesome names for the equivalent Lucide
 * component without touching every call-site in the same PR.
 *
 * The mapping mirrors the table in devdocs/frontend/icons.md. Every
 * alias is a direct re-export — tree-shaking is preserved because
 * lucide-vue-next ships one component per module.
 *
 * Slated for removal in Phase 6 (#1331). Do not add new entries here
 * unless you are migrating existing code; new code uses Lucide names.
 */

export {
  // Commodity types
  Box as FaBox,
  CookingPot as FaBlender,
  Laptop as FaLaptop,
  Wrench as FaTools,
  Sofa as FaCouch,
  Shirt as FaTshirt,

  // File types
  FileText as FaFilePdf,
  FileText as FaFileAlt,
  FileImage as FaFileImage,
  Book as FaBook,
  Receipt as FaFileInvoiceDollar,
  File as FaFile,
  FileDown as FaFileExport,

  // Actions
  Download as FaDownload,
  Trash2 as FaTrash,
  X as FaTimes,
  Pencil as FaPencilAlt,
  Pencil as FaEdit,
  Check as FaCheck,
  Printer as FaPrint,
  Plus as FaPlus,
  Search as FaSearch,
  Eye as FaEye,
  Info as FaInfoCircle,
  ArrowLeft as FaArrowLeft,
  ArrowRight as FaArrowRight,
  Quote as FaQuoteLeftAlt,
  MapPin as FaMapMarkerAlt,
  ChevronDown as FaChevronDown,
  ChevronRight as FaChevronRight,
  ChevronLeft as FaChevronLeft,
  ChevronUp as FaChevronUp,
  ZoomOut as FaSearchMinus,
  ZoomIn as FaSearchPlus,
  Copy as FaCopy,
  Upload as FaUpload,
  UploadCloud as FaCloudUploadAlt,
  AlertTriangle as FaExclamationTriangle,
  AlertCircle as FaExclamationCircle,
  CheckCircle2 as FaCheckCircle,
  Loader2 as FaSpinner,
  RotateCw as FaRedo,
  Calendar as FaCalendar,
  Image as FaImage,
  Video as FaVideo,
  Music as FaMusic,
  Archive as FaArchive,
  Save as FaSave,
  ExternalLink as FaExternalLinkAlt,
  Lock as FaLock,
  Ban as FaBan,
  User as FaUser,
  UserPlus as FaUserPlus,
  LogOut as FaRightFromBracket,
} from 'lucide-vue-next'
